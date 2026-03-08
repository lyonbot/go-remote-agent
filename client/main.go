package client

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"remote-agent/biz"
	"remote-agent/utils"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type portForward struct {
	localPort  int
	remoteAddr string
	remotePort int
}

func parsePortForward(s string) (portForward, error) {
	// Accepts: localPort:remoteAddr:remotePort  or  localPort:remotePort (→ localhost)
	parts := strings.SplitN(s, ":", 3)
	switch len(parts) {
	case 2:
		lp, err := strconv.Atoi(parts[0])
		if err != nil {
			return portForward{}, fmt.Errorf("invalid local port %q", parts[0])
		}
		rp, err := strconv.Atoi(parts[1])
		if err != nil {
			return portForward{}, fmt.Errorf("invalid remote port %q", parts[1])
		}
		return portForward{localPort: lp, remoteAddr: "localhost", remotePort: rp}, nil
	case 3:
		lp, err := strconv.Atoi(parts[0])
		if err != nil {
			return portForward{}, fmt.Errorf("invalid local port %q", parts[0])
		}
		rp, err := strconv.Atoi(parts[2])
		if err != nil {
			return portForward{}, fmt.Errorf("invalid remote port %q", parts[2])
		}
		return portForward{localPort: lp, remoteAddr: parts[1], remotePort: rp}, nil
	default:
		return portForward{}, fmt.Errorf("expected localPort:remoteAddr:remotePort, got %q", s)
	}
}

// localConn tracks one forwarded TCP connection.
type localConn struct {
	conn    net.Conn
	ready   chan struct{} // closed when dial result received from agent
	dialErr string       // non-empty if dial failed
}

type clientState struct {
	ws      *utils.RWChan
	counter atomic.Uint32
	conns   sync.Map // map[uint32]*localConn
}

func Run() {
	cfg := biz.Config

	// Parse port-forward specs
	pfs := make([]portForward, 0, len(cfg.ClientForwards))
	for _, spec := range cfg.ClientForwards {
		pf, err := parsePortForward(spec)
		if err != nil {
			log.Fatalf("invalid -L %q: %v", spec, err)
		}
		pfs = append(pfs, pf)
	}

	// Build WebSocket URL from server base URL
	wsURL := cfg.BaseUrl
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/api/agent/" + cfg.Name + "/omni/"

	headers := http.Header{}
	if cfg.APIKey != "" {
		headers.Set("X-API-Key", cfg.APIKey)
	}

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Insecure},
	}
	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		log.Fatalf("failed to connect to %s: %v", wsURL, err)
	}
	log.Printf("connected to %s (agent: %s)", cfg.BaseUrl, cfg.Name)

	c := &clientState{ws: utils.MakeRWChanFromWebSocket(conn)}

	go c.demux()

	// Start a local TCP listener for each -L spec.
	// Block until all listeners exit (i.e. until the WS connection drops).
	wg := sync.WaitGroup{}
	for _, pf := range pfs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.listenLocal(pf)
		}()
	}
	wg.Wait()
}

// demux reads from the agent WebSocket and dispatches to the right localConn.
func (c *clientState) demux() {
	for data := range c.ws.Read {
		if len(data) < 1 {
			continue
		}
		switch data[0] {
		case 0xff: // debug message from agent
			log.Printf("[agent] %s", string(data[1:]))

		case 0x20: // dial result: [0x20][id:4][errCode:1][message]
			if len(data) < 6 {
				continue
			}
			id := binary.LittleEndian.Uint32(data[1:5])
			errCode := data[5]
			msg := string(data[6:])
			if v, ok := c.conns.Load(id); ok {
				lc := v.(*localConn)
				if errCode != 0x00 {
					lc.dialErr = msg
				}
				close(lc.ready)
			}

		case 0x21: // data: [0x21][id:4][payload]
			if len(data) < 6 {
				continue
			}
			id := binary.LittleEndian.Uint32(data[1:5])
			payload := data[5:]
			if v, ok := c.conns.Load(id); ok {
				v.(*localConn).conn.Write(payload)
			}

		case 0x22: // close: [0x22][id:4]
			if len(data) < 5 {
				continue
			}
			id := binary.LittleEndian.Uint32(data[1:5])
			if v, ok := c.conns.LoadAndDelete(id); ok {
				v.(*localConn).conn.Close()
			}
		}
	}

	// WS closed — close all open local connections
	log.Println("agent connection closed, shutting down")
	c.conns.Range(func(_, v any) bool {
		v.(*localConn).conn.Close()
		return true
	})
}

func (c *clientState) listenLocal(pf portForward) {
	addr := fmt.Sprintf("127.0.0.1:%d", pf.localPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}
	defer ln.Close()
	log.Printf("forwarding 127.0.0.1:%d -> %s:%d (via agent)", pf.localPort, pf.remoteAddr, pf.remotePort)

	// Close listener when WebSocket disconnects
	go func() {
		<-c.ws.Ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go c.handleConn(conn, pf)
	}
}

func (c *clientState) handleConn(conn net.Conn, pf portForward) {
	id := c.counter.Add(1)
	idBytes := binary.LittleEndian.AppendUint32(nil, id)

	lc := &localConn{conn: conn, ready: make(chan struct{})}
	c.conns.Store(id, lc)
	defer c.conns.CompareAndDelete(id, lc)
	defer conn.Close()

	// Send open packet: [0x20][id:4][port:2][addr]
	portBytes := binary.LittleEndian.AppendUint16(nil, uint16(pf.remotePort))
	c.ws.Write(utils.JoinBytes2(0x20, idBytes, portBytes, []byte(pf.remoteAddr)))

	// Wait for dial result
	<-lc.ready
	if lc.dialErr != "" {
		log.Printf("[0x%x] dial %s:%d failed: %s", id, pf.remoteAddr, pf.remotePort, lc.dialErr)
		c.ws.Write(utils.JoinBytes2(0x22, idBytes))
		return
	}

	// Forward local TCP → agent
	buf := make([]byte, 32*1024)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			c.ws.Write(utils.JoinBytes2(0x21, idBytes, buf[:n]))
		}
		if err != nil {
			break
		}
	}

	// Notify agent that client side closed
	c.ws.Write(utils.JoinBytes2(0x22, idBytes))
}
