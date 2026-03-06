package proxy

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"remote-agent/biz"
	"strings"
	"sync"
	"sync/atomic"
)

type ServiceInfo struct {
	Host      string `json:"host"`
	AgentName string `json:"agent_name"`
	AgentId   string `json:"agent_id"`

	Target      string `json:"target"`
	ReplaceHost string `json:"replace_host"`
}

type Service struct {
	ServiceInfo

	ctx    context.Context
	cancel context.CancelFunc
	connMu sync.Mutex
	conn   atomic.Pointer[ConnectionToAgent]
}

func (s *Service) ensureConnected() (c *ConnectionToAgent, err error) {
	// Fast path: connection already established and ready.
	if c = s.conn.Load(); c != nil {
		if err = c.WaitForReady(); err == nil {
			return c, nil
		}
		s.conn.CompareAndSwap(c, nil)
	}

	// Slow path: create a new connection under lock to prevent duplicate goroutines.
	s.connMu.Lock()
	defer s.connMu.Unlock()

	// Re-check after acquiring the lock.
	if c = s.conn.Load(); c != nil {
		if err = c.WaitForReady(); err == nil {
			return c, nil
		}
		s.conn.CompareAndSwap(c, nil)
	}

	c = NewConnectionToAgent(s.ctx)
	s.conn.Store(c)
	log.Printf("[proxy '%s'] agent connection created", s.Host)
	go c.ConnectAndCommunicate(s.AgentName, s.AgentId, func(connErr error) {
		log.Printf("[proxy '%s'] agent connection closed: %s", s.Host, connErr.Error())
		s.conn.CompareAndSwap(c, nil)
	})

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
		targetURL := s.Target + r.URL.EscapedPath()
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}

		isWebSocket := strings.ToLower(r.Header.Get("Upgrade")) == "websocket"
		if isWebSocket {
			// Replace scheme via url.Parse to avoid corrupting hostnames containing "http".
			parsed, parseErr := url.Parse(targetURL)
			if parseErr != nil {
				return nil, parseErr
			}
			switch parsed.Scheme {
			case "http":
				parsed.Scheme = "ws"
			case "https":
				parsed.Scheme = "wss"
			}
			targetURL = parsed.String()
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
			URL:     targetURL,
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
