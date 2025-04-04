package proxy

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"remote-agent/biz"
	"remote-agent/server/agent_handler"
	"remote-agent/utils"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type ConnectionToAgent struct {
	Ctx context.Context

	agentName   string
	counter     atomic.Uint32 // connection counter
	chanToAgent chan<- []byte // send to agent
	R           sync.Map      // map[uint32]<-chan []byte // receive from agent

	mu              sync.Mutex
	cond            *sync.Cond
	status          CTAStatus
	connectionError error
}

type CTAStatus int

const (
	CTAStatusConnecting CTAStatus = iota
	CTAStatusConnected
	CTAStatusDisconnected
)

func NewConnectionToAgent(ctx context.Context) *ConnectionToAgent {
	c := &ConnectionToAgent{
		Ctx: ctx,
	}
	c.cond = sync.NewCond(&c.mu)

	return c
}

// run this in a goroutine when connection created.
func (c *ConnectionToAgent) ConnectAndCommunicate(agent_name string, agent_id string, onDisconnected func(error)) {
	err := c.communicate(agent_name, agent_id)
	defer onDisconnected(err)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = CTAStatusDisconnected
	c.connectionError = err
	c.cond.Broadcast()
}

// (internal) a goroutine that talks to agent. it always returns a error, when connection is closed.
// when connected, it will update c.status and notify c.cond. but when disconnected, it wont update c.status.
func (c *ConnectionToAgent) communicate(agent_name string, agent_id string) error {
	c.agentName = agent_name

	tunnel, err := agent_handler.MakeAgentTunnel(agent_name, agent_id)
	if err != nil {
		return err
	}

	C_from_agent := tunnel.ChFromAgent
	C_to_agent := tunnel.ChToAgent
	defer tunnel.Close()

	// notify agent
	if err := tunnel.NotifyAgent(biz.AgentNotify{
		Type: "pty",
	}); err != nil {
		return err
	}

	// send a ping
	ping := []byte{0xff, 'h', 'i'}
	C_to_agent <- ping
	select {
	case <-c.Ctx.Done():
		return errors.New("connection aborted by parent context")
	case <-time.After(time.Second * 5):
		return errors.New("connection ping timeout")
	case pong := <-C_from_agent:
		if !bytes.Equal(ping, pong) {
			return errors.New("ping failed. got " + hex.EncodeToString(pong))
		}
	}

	// ready!
	c.chanToAgent = C_to_agent

	c.mu.Lock()
	c.status = CTAStatusConnected
	c.cond.Broadcast()
	c.mu.Unlock()

	// a watchdog will kill the connection if it doesn't respond in 10 minutes
	isAboutToClean := false
	killInMinutes := 10 * time.Minute
	watchdogTicker := time.NewTicker(killInMinutes / 2)
	defer watchdogTicker.Stop()

	for {
		select {
		case <-watchdogTicker.C:
			if isAboutToClean {
				return errors.New("watchdog timeout")
			} else {
				isAboutToClean = true
			}

		case data, ok := <-C_from_agent:
			if len(data) >= 1 && data[0] == 0xff {
				log.Printf("[agent '%s'] message: %s", agent_name, string(data[1:]))
			} else if len(data) >= 5 {
				idBytes := data[1:5]
				id := binary.LittleEndian.Uint32(idBytes)
				if ch, ok := c.R.Load(id); ok && ch != nil {
					isAboutToClean = false
					ch.(chan []byte) <- data
				} else {
					C_to_agent <- utils.JoinBytes2(0x22, idBytes) // close connection
					log.Printf("[agent '%s'] bad proxy reqId 0x%x with package 0x%x", agent_name, id, data[0])
				}
			}
			if !ok {
				return errors.New("tunnel from agent closed")
			}

		case <-c.Ctx.Done():
			return c.Ctx.Err()
		}
	}
}

// wait and ensure connection ready. if connection closed or failed, it will return an error.
func (c *ConnectionToAgent) WaitForReady() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// wait for connection built. then c.status will be CTAStatusConnected, or CTAStatusDisconnected
	for c.status == CTAStatusConnecting {
		c.cond.Wait()
	}

	if c.status == CTAStatusConnected {
		// good to go
		return nil
	}

	err := c.connectionError
	if err == nil {
		err = fmt.Errorf("bad connection status: %d", c.status)
	}
	return err
}

func stripHeaders(headers []biz.ProxyHttpHeader) []biz.ProxyHttpHeader {
	ans := make([]biz.ProxyHttpHeader, 0, len(headers))
	for _, header := range headers {
		name := strings.ToLower(header.Name)
		if (name == "connection" || name == "upgrade" || name == "keep-alive") ||
			(name == "proxy-connection" || name == "proxy-authorization") ||
			(name == "sec-websocket-key" || name == "sec-websocket-version" || name == "sec-websocket-extensions" || name == "sec-websocket-accept") {
			continue
		}
		ans = append(ans, header)
	}
	return ans
}

// proxy HTTP request from client to remote via agent.
// if failed to build a connection, it will return an error, and you shall send "Bad Gateway" response to client when error presents.
func (c *ConnectionToAgent) HandleRequest(connReq *biz.ProxyHttpRequest, w http.ResponseWriter, r *http.Request) error {
	id := c.counter.Add(1)
	idBytes := binary.LittleEndian.AppendUint32(nil, id)

	log.Printf("[agent '%s'] request %x: %s %s", c.agentName, id, connReq.Method, connReq.URL)

	connReq.Headers = stripHeaders(connReq.Headers)

	chanToAgent := c.chanToAgent
	chanFromAgent := make(chan []byte, 5)
	c.R.Store(id, chanFromAgent)
	defer c.R.CompareAndDelete(id, chanFromAgent)

	connReqBytes, _ := connReq.MarshalMsg(nil)
	chanToAgent <- utils.JoinBytes2(0x23, idBytes, connReqBytes)

	connRes := biz.ProxyHttpResponse{}
	select { // try to establish a connection on agent
	case <-c.Ctx.Done():
		return c.Ctx.Err()

	case <-time.After(time.Second * 60):
		return errors.New("timeout")

	case recv := <-chanFromAgent:
		if recv[0] != 0x23 {
			return errors.New("bad response: expect 0x23 package")
		}
		if _, err := connRes.UnmarshalMsg(recv[5:]); err != nil {
			return errors.New("bad response: " + err.Error())
		}
		if connRes.ConnectionError != "" {
			return errors.New("connection error: " + connRes.ConnectionError)
		}
	}

	// ---- websocket connection!
	if connRes.IsWebSocket {
		connRes.Headers = stripHeaders(connRes.Headers)
		wsConn, err := ws.Upgrade(w, r, biz.ToHttpRequestHeaders(connRes.Headers))
		if err != nil {
			return err
		}

		wsConn.SetPingHandler(func(appData string) error {
			chanToAgent <- (utils.JoinBytes2(0x21, idBytes, []byte{0x09}, []byte(appData)))
			return nil
		})
		wsConn.SetPongHandler(func(appData string) error {
			chanToAgent <- (utils.JoinBytes2(0x21, idBytes, []byte{0x0a}, []byte(appData)))
			return nil
		})

		wsConnClosed := make(chan struct{}, 1)
		wg := sync.WaitGroup{}

		wg.Add(1)
		go func() { // agent->http
			defer wg.Done()

			for {
				select {
				case <-wsConnClosed:
					return
				case <-c.Ctx.Done():
					wsConn.Close()
					return
				case recv := <-chanFromAgent:
					if recv[0] == 0x21 {
						messageType := int(recv[5])
						wsConn.WriteMessage(messageType, recv[6:])
					}
					if recv[0] == 0x22 {
						wsConn.Close()
						return
					}
				}
			}
		}()

		wg.Add(1)
		go func() { // http->agent
			defer wg.Done()

			for {
				messageType, data, err := wsConn.ReadMessage()
				if err != nil {
					log.Printf("[agent '%s'] ws 0x%x aborted by client", c.agentName, id)
					break
				}

				chanToAgent <- utils.JoinBytes2(0x21, idBytes, []byte{uint8(messageType)}, data)
			}

			// client disconnected. shall close the proxy too
			chanToAgent <- utils.JoinBytes2(0x22, idBytes)
			wsConnClosed <- struct{}{}
		}()

		wg.Wait()
		wsConn.Close()
		log.Printf("[agent '%s'] ws 0x%x totally closed", c.agentName, id)
		return nil
	}

	// ---- regular http request
	{ // write response header & status code
		for _, h := range connRes.Headers {
			w.Header().Add(h.Name, h.Value)
		}
		w.WriteHeader(int(connRes.StatusCode))
	}

	{ // write body data
		for {
			select {
			case <-r.Context().Done():
				// http request disconnected
				log.Printf("[agent '%s'] request %x aborted by client", c.agentName, id)
				chanToAgent <- utils.JoinBytes2(0x22, idBytes)
				return nil
			case <-c.Ctx.Done():
				// agent disconnected
				return nil
			case data := <-chanFromAgent:
				if data[0] == 0x22 {
					// close connection
					return nil
				}
				if data[0] == 0x21 {
					// data
					_, err := w.(io.Writer).Write(data[5:])
					w.(http.Flusher).Flush()
					if err != nil {
						// connection closed by client?
						log.Printf("[agent '%s'] request %x met write error: %s", c.agentName, id, err.Error())
						chanToAgent <- utils.JoinBytes2(0x22, idBytes)
						return nil
					}
				}
			}
		}
	}
}

var ws = websocket.Upgrader{
	EnableCompression: true,
	CheckOrigin: func(r *http.Request) bool {
		// accept all origin -- be good with reverse proxies
		return true
	},
}
