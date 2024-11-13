package proxy

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
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

type Service struct {
	Host      string `json:"host"`
	AgentName string `json:"agent_name"`
	AgentId   string `json:"agent_id"`

	Target      string `json:"target"`
	ReplaceHost string `json:"replace_host"`

	ctx    context.Context
	cancel context.CancelFunc
	conn   *connectionToAgent
}

func (s *Service) ensureConnected() (c *connectionToAgent, err error) {
	c = s.conn

	if c == nil {
		// create new connection
		ctx, cancel := context.WithCancel(s.ctx)

		c = newConnectionToAgent(ctx)
		err := c.connect(s.AgentName, s.AgentId, func(err error) {
			cancel()
			if c == s.conn {
				s.conn = nil
			}
		})
		if err != nil {
			return nil, err
		}

		s.conn = c
	}

	// wait for ready
	if err = c.waitForReady(); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) HandleRequest(w http.ResponseWriter, r *http.Request) {
	c, err := s.ensureConnected()
	if err != nil {
		w.Header().Add("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(err.Error()))
		return
	}

	bizRequest, err := func() (*biz.ProxyHttpRequest, error) {
		url := s.Target + r.URL.EscapedPath()
		if r.URL.RawQuery != "" {
			url += "?" + r.URL.RawQuery
		}

		isWebSocket := r.Header.Get("Upgrade") == "websocket"
		if isWebSocket {
			url = strings.Replace(url, "http", "ws", 1)
		}

		headers := biz.FromHttpRequestHeaders(r.Header)

		var body []byte
		if !isWebSocket && r.Body != nil {
			if r.ContentLength > 0 {
				body = make([]byte, r.ContentLength)
				if n, err := r.Body.Read(body); err != nil && int64(n) != r.ContentLength {
					return nil, err
				}
			} else {
				body, err = io.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}
			}
		}

		bizRequest := &biz.ProxyHttpRequest{
			Method:  r.Method,
			URL:     url,
			Headers: headers,
			Host:    s.ReplaceHost,
			Body:    body,
		}
		return bizRequest, nil
	}()
	if err != nil {
		w.Header().Add("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	if err := c.HandleRequest(bizRequest, w, r); err != nil {
		w.Header().Add("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(err.Error()))
		return
	}
}

func (s *Service) Dispose() {
	s.cancel()
}

// -----------------------------------------------------------------------------

type connectionToAgent struct {
	Ctx context.Context

	counter     atomic.Uint32 // connection counter
	chanToAgent chan<- []byte // send to agent
	R           sync.Map      // map[uint32]<-chan []byte // receive from agent

	mu        sync.Mutex
	cond      *sync.Cond
	connected bool
}

func newConnectionToAgent(ctx context.Context) *connectionToAgent {
	ans := &connectionToAgent{
		Ctx: ctx,
	}
	ans.cond = sync.NewCond(&ans.mu)
	return ans
}

func (c *connectionToAgent) connect(agent_name string, agent_id string, onDisconnected func(error)) error {
	tunnel, _, _, notifyAgent, C_to_agent, C_from_agent, err := agent_handler.MakeAgentTunnel(agent_name, agent_id)
	if err != nil {
		onDisconnected(err)
		return err
	}

	disconnect := func(err error) {
		tunnel.Delete()
		close(C_to_agent)
		onDisconnected(err)
	}

	// notify agent
	if err := notifyAgent(biz.AgentNotify{
		Type: "pty",
	}); err != nil {
		disconnect(err)
		return err
	}

	// send a ping
	ping := []byte{0xff, 'h', 'i'}
	C_to_agent <- ping
	select {
	case <-c.Ctx.Done():
		err := errors.New("parent context canceled")
		disconnect(err)
		return err
	case <-time.After(time.Second * 5):
		err := errors.New("timeout")
		disconnect(err)
		return err
	case pong := <-C_from_agent:
		if !bytes.Equal(ping, pong) {
			err := errors.New("ping failed. got " + hex.EncodeToString(pong))
			disconnect(err)
			return err
		}
	}

	// ready!
	c.chanToAgent = C_to_agent

	go func() {
		defer disconnect(nil)

		isAboutToClean := false
		watchdogTicker := time.NewTicker(time.Minute * 5) // duration * 2 = kick timeout
		defer watchdogTicker.Stop()

		for {
			select {
			case <-watchdogTicker.C:
				if isAboutToClean {
					disconnect(errors.New("watchdog timeout"))
					return
				} else {
					isAboutToClean = true
				}

			case data, ok := <-C_from_agent:
				if len(data) >= 5 {
					isAboutToClean = false
					idBytes := data[1:5]
					id := binary.BigEndian.Uint32(idBytes)
					if ch, ok := c.R.Load(id); ok && ch != nil {
						ch.(chan []byte) <- data
					}
				}
				if !ok {
					return
				}

			case <-c.Ctx.Done():
				return
			}
		}
	}()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = true
	c.cond.Broadcast()
	return nil
}

func (c *connectionToAgent) waitForReady() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for !c.connected {
		done := make(chan struct{})
		go func() {
			select {
			case <-time.After(time.Second * 5):
				err = errors.New("timeout")
				c.cond.Broadcast()
			case <-c.Ctx.Done():
				err = c.Ctx.Err()
				c.cond.Broadcast()
			case <-done:
			}
		}()

		c.cond.Wait()
		close(done)
	}
	return err
}

// proxy HTTP request from client to remote via agent.
// if failed to build a connection, it will return an error, and you shall send "Bad Gateway" response to client when error presents.
func (c *connectionToAgent) HandleRequest(connReq *biz.ProxyHttpRequest, w http.ResponseWriter, r *http.Request) error {
	id := c.counter.Add(1)
	idBytes := binary.BigEndian.AppendUint32(nil, id)

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
		wsConn, err := ws.Upgrade(w, r, biz.ToHttpRequestHeaders(connRes.Headers))
		if err != nil {
			return err
		}

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
				messageType, r, err := wsConn.NextReader()
				if err != nil {
					break
				}

				data, err := io.ReadAll(r)
				if err != nil {
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
					w.(io.Writer).Write(data[5:])
					w.(http.Flusher).Flush()
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
