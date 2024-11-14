package agent_omni

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"remote-agent/biz"
	"remote-agent/utils"
	"sync"

	"github.com/gorilla/websocket"
)

func (s *PtySession) SetupProxy() {
	type ProxyChannel struct {
		FromUser chan<- []byte // data from user. user may close this to close the connection
	}
	channels := sync.Map{} // map[uint32]*ProxyChannel

	// open tcp proxy channel
	s.Handlers[0x20] = func(recv []byte) {
		if len(recv) < 7 {
			s.WriteDebugMessage("invalid tcp proxy channel request")
			return
		}

		idBytes := recv[1:5]
		id := binary.LittleEndian.Uint32(idBytes)
		port := binary.LittleEndian.Uint16(recv[5:])
		addr := string(recv[7:])

		send_dial_result := func(err_code byte, msg string) {
			s.Write(utils.JoinBytes2(0x20, idBytes, []byte{err_code}, []byte(msg)))
		}

		channel := &ProxyChannel{}
		if _, exists := channels.LoadOrStore(id, channel); exists {
			send_dial_result(0x01, "connection id already exists")
			s.WriteDebugMessage(fmt.Sprintf("tcp proxy 0x%x already opened", id))
			return
		}

		go func() {
			defer channels.CompareAndDelete(id, channel)

			// tcp proxy
			conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", addr, port))
			if err != nil {
				send_dial_result(0x01, "dial error: "+err.Error())
				return
			}

			// ---- connection ok.
			defer conn.Close()
			defer s.Write(utils.JoinBytes2(0x22, idBytes)) // close connection

			wg := sync.WaitGroup{}
			defer wg.Wait()

			// ---- continuous read data
			C_user_data := make(chan []byte, 5)
			ctx, stop := context.WithCancel(s.Ctx)
			channel.FromUser = C_user_data

			wg.Add(1) // handle data from user
			go func() {
				defer wg.Done()
				defer conn.Close() // if session end, or user close connection, close tcp connection

				stopped_by_ctx := utils.ReadChanUnderContext(ctx, C_user_data, func(data []byte) {
					conn.Write(data)
				})
				channel.FromUser = nil
				if stopped_by_ctx {
					close(C_user_data)
				}
			}()

			wg.Add(1) // handle data from remote
			go func() {
				defer wg.Done()
				defer stop()

				// ---- ready for data transfer
				send_dial_result(0x00, conn.LocalAddr().String())

				// ---- continuous send to user
				for {
					data := make([]byte, 1024)
					n, err := conn.Read(data)
					if n > 0 {
						s.Write(utils.JoinBytes2(0x21, idBytes, data[:n]))
					}
					if err != nil {
						return
					}
				}
			}()
		}()
	}

	// write tcp proxy channel
	s.Handlers[0x21] = func(recv []byte) {
		if len(recv) <= 5 {
			s.WriteDebugMessage("invalid tcp proxy channel request")
			return
		}

		idBytes := recv[1:5]
		id := binary.LittleEndian.Uint32(idBytes)

		if val, ok := channels.Load(id); ok {
			channel := val.(*ProxyChannel)
			utils.TryWrite(channel.FromUser, recv[5:])
		} else {
			s.WriteDebugMessage(fmt.Sprintf("tcp proxy 0x%x not found", id))
		}
	}

	// close tcp proxy channel
	s.Handlers[0x22] = func(recv []byte) {
		if len(recv) < 5 {
			s.WriteDebugMessage("invalid tcp proxy channel request")
			return
		}

		idBytes := recv[1:5]
		id := binary.LittleEndian.Uint32(idBytes)

		if val, ok := channels.Load(id); ok {
			channel := val.(*ProxyChannel)
			utils.TryClose(channel.FromUser)
			channel.FromUser = nil
		} else {
			s.WriteDebugMessage(fmt.Sprintf("tcp proxy 0x%x not found", id))
		}
	}

	// create http proxy channel
	s.Handlers[0x23] = func(recv []byte) {
		idBytes := recv[1:5]
		id := binary.LittleEndian.Uint32(idBytes)
		reqBytes := recv[5:]

		dial_result := &biz.ProxyHttpResponse{}
		send_dial_result := func() {
			resBytes, _ := dial_result.MarshalMsg(nil)
			s.Write(utils.JoinBytes2(0x23, idBytes, resBytes))
		}

		channel := &ProxyChannel{}
		if _, exists := channels.LoadOrStore(id, channel); exists {
			dial_result.ConnectionError = "connection id already exists"
			send_dial_result()
			return
		}

		// ----------------------------------------------
		// id verify ok, start connect, and update `proxy`

		go func() {
			defer channels.CompareAndDelete(id, channel)

			// prepare req
			is_websocket := false
			req := &biz.ProxyHttpRequest{}
			{
				if _, err := req.UnmarshalMsg(reqBytes); err != nil {
					dial_result.ConnectionError = "bad request: " + err.Error()
					send_dial_result()
					return
				}

				url_parsed, err := url.Parse(req.URL)
				if err != nil {
					dial_result.ConnectionError = "bad url: " + err.Error()
					send_dial_result()
					return
				}

				is_websocket = url_parsed.Scheme == "ws" || url_parsed.Scheme == "wss"
			}

			// websocket
			if is_websocket {
				dialer := websocket.Dialer{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: biz.Config.Insecure,
					},
				}

				if req.Host != "" {
					// see https://github.com/gorilla/websocket/commit/6fd0f867fef40c540fa05c59f86396de10a632a6#diff-4b667feae66c9d46b21b9ecc19e8958cf4472d162ce0a47ac3e8386af8bbd8cfR234
					req.Headers = append(req.Headers, biz.ProxyHttpHeader{
						Name:  "Host",
						Value: req.Host,
					})
				}
				conn, resp, err := dialer.Dial(req.URL, biz.ToHttpRequestHeaders(req.Headers))
				if err != nil {
					dial_result.ConnectionError = "connect error: " + err.Error()
					send_dial_result()
					return
				}

				// --- success conn
				defer conn.Close()
				defer s.Write(utils.JoinBytes2(0x22, idBytes)) // close connection
				if s.Ctx.Err() != nil {                        // double check if "omni" session still there
					return
				}

				wg := sync.WaitGroup{}
				defer wg.Wait()

				conn.SetPingHandler(func(appData string) error {
					s.Write(utils.JoinBytes2(0x21, idBytes, []byte{0x09}, []byte(appData)))
					return nil
				})
				conn.SetPongHandler(func(appData string) error {
					s.Write(utils.JoinBytes2(0x21, idBytes, []byte{0x0a}, []byte(appData)))
					return nil
				})

				// ---- continuous read data
				C_user_data := make(chan []byte, 5)
				ctx, stop := context.WithCancel(s.Ctx)
				channel.FromUser = C_user_data

				wg.Add(1) // handle data from user
				go func() {
					defer wg.Done()
					defer conn.Close() // if session end, or user close connection, close ws connection

					stopped_by_ctx := utils.ReadChanUnderContext(ctx, C_user_data, func(data []byte) {
						if len(data) >= 1 {
							messageType := int(data[0])
							conn.WriteMessage(messageType, data[1:])
						}
					})
					channel.FromUser = nil
					if stopped_by_ctx {
						close(C_user_data)
					}
				}()

				wg.Add(1) // handle data from remote
				go func() {
					defer wg.Done()
					defer stop()

					// ---- ready for data transfer
					dial_result.IsWebSocket = true
					dial_result.StatusCode = int32(resp.StatusCode)
					dial_result.Headers = biz.FromHttpRequestHeaders(resp.Header)
					send_dial_result()

					// ---- continuous send to user
					for {
						messageType, r, err := conn.NextReader()
						if err != nil {
							return
						}

						data, err := io.ReadAll(r)
						if err != nil {
							return
						}
						s.Write(utils.JoinBytes2(0x21, idBytes, []byte{uint8(messageType)}, data))
					}
				}()
			}

			// regular http request
			if !is_websocket {
				if req.Body == nil {
					req.Body = make([]byte, 0)
				}

				httpReqBodyReader := bytes.NewReader(req.Body)
				httpCtx, cancelHttpCtx := context.WithCancel(s.Ctx)
				httpReq, err := http.NewRequestWithContext(httpCtx, req.Method, req.URL, httpReqBodyReader)

				if err != nil {
					dial_result.ConnectionError = "bad request: " + err.Error()
					send_dial_result()
					cancelHttpCtx()
					return
				}

				// --------

				wg := sync.WaitGroup{}
				defer wg.Wait()

				{ // handle data from user -- wait for "0x22" close-connection package
					C_user_data := make(chan []byte, 5)
					channel.FromUser = C_user_data
					wg.Add(1)
					go func() {
						defer wg.Done()

						stopped_by_ctx := utils.ReadChanUnderContext(httpCtx, C_user_data, func(data []byte) {
							// nothing to do -- http request is not duplex
						})
						channel.FromUser = nil
						if stopped_by_ctx {
							close(C_user_data)
						} else {
							cancelHttpCtx()
						}
					}()
				}

				httpReq.Host = req.Host
				httpReq.Header = biz.ToHttpRequestHeaders(req.Headers)
				httpRes, err := http.DefaultClient.Do(httpReq)
				if err != nil {
					dial_result.ConnectionError = "connect error: " + err.Error()
					send_dial_result()
					return
				}

				// ---- connection ok.
				httpResBody := httpRes.Body

				defer httpResBody.Close()
				defer s.Write(utils.JoinBytes2(0x22, idBytes)) // close connection
				defer wg.Wait()

				wg.Add(1) // handle data from remote
				go func() {
					defer wg.Done()
					defer cancelHttpCtx()

					// ---- ready for data transfer
					dial_result.StatusCode = int32(httpRes.StatusCode)
					dial_result.Headers = biz.FromHttpRequestHeaders(httpRes.Header)
					send_dial_result()

					// ---- continuous send to user
					for {
						data := make([]byte, 1024)
						n, err := httpResBody.Read(data)
						if n > 0 {
							s.Write(utils.JoinBytes2(0x21, idBytes, data[:n]))
						}
						if err != nil {
							return
						}
					}
				}()
			}
		}()
	}
}
