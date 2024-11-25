package proxy

import (
	"context"
	"io"
	"log"
	"net/http"
	"remote-agent/biz"
	"strings"
	"sync/atomic"
)

type Service struct {
	Host      string `json:"host"`
	AgentName string `json:"agent_name"`
	AgentId   string `json:"agent_id"`

	Target      string `json:"target"`
	ReplaceHost string `json:"replace_host"`

	ctx    context.Context
	cancel context.CancelFunc
	conn   atomic.Pointer[ConnectionToAgent]
}

func (s *Service) ensureConnected() (c *ConnectionToAgent, err error) {
	c = s.conn.Load()

	if c == nil {
		// create new connection
		c = NewConnectionToAgent(s.ctx)
		s.conn.CompareAndSwap(nil, c)

		log.Printf("[proxy '%s'] agent connection created", s.Host)
		go c.ConnectAndCommunicate(s.AgentName, s.AgentId, func(err error) {
			log.Printf("[proxy '%s'] agent connection closed: %s", s.Host, err.Error())
			s.conn.CompareAndSwap(c, nil)
		})
	}

	// wait for ready
	if err = c.WaitForReady(); err != nil {
		s.conn.CompareAndSwap(c, nil)
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

		isWebSocket := strings.ToLower(r.Header.Get("Upgrade")) == "websocket"
		if isWebSocket {
			url = strings.Replace(url, "http", "ws", 1)
		}

		headers := biz.FromHttpRequestHeaders(r.Header)

		var body []byte
		if !isWebSocket && r.Body != nil {
			if r.ContentLength > 0 {
				bytesRead := int64(0)
				body = make([]byte, r.ContentLength)
				for {
					n, err := r.Body.Read(body[bytesRead:])
					bytesRead += int64(n)
					if bytesRead == r.ContentLength {
						break
					}
					if err != nil {
						return nil, err
					}
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
		log.Printf("[proxy '%s'] error %s %s: %s", s.Host, bizRequest.Method, bizRequest.URL, err.Error())
		w.Header().Add("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(err.Error()))
		return
	}
}

// close the connections to agent
func (s *Service) Dispose() {
	s.cancel()
}
