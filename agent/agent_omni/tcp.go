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

func (s *PtySession) SetupTcpProxy() {
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
					if err != nil {
						return
					}

					if n > 0 {
						s.Write(utils.JoinBytes2(0x21, idBytes, data[:n]))
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

			req := &biz.ProxyHttpRequest{}
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

			req_header := http.Header{}
			for _, header := range req.Headers {
				req_header.Set(header.Name, header.Value)
			}

			is_websocket := url_parsed.Scheme == "ws" || url_parsed.Scheme == "wss"
			if is_websocket {
				dialer := websocket.Dialer{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: biz.Config.Insecure,
					},
				}

				conn, resp, err := dialer.Dial(req.URL, req_header)
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
					dial_result.StatusCode = int32(resp.StatusCode)
					for h := range resp.Header {
						dial_result.Headers = append(dial_result.Headers, biz.ProxyHttpHeader{
							Name:  h,
							Value: resp.Header.Get(h),
						})
					}
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
						s.Write(utils.JoinBytes2(0x24, idBytes, []byte{uint8(messageType)}, data))
					}
				}()
			}

			// regular http request, one-way data
			if !is_websocket {
				req_body_reader := bytes.NewReader(req.Body)
				http_req, err := http.NewRequestWithContext(s.Ctx, req.Method, req.URL, req_body_reader)

				if err != nil {
					dial_result.ConnectionError = "bad request: " + err.Error()
					send_dial_result()
					return
				}

				http_req.Header = req_header
				http_res, err := http.DefaultClient.Do(http_req)
				if err != nil {
					dial_result.ConnectionError = "connect error: " + err.Error()
					send_dial_result()
					return
				}

				// ---- connection ok.
				conn := http_res.Body
				defer conn.Close()
				defer s.Write(utils.JoinBytes2(0x22, idBytes)) // close connection

				wg := sync.WaitGroup{}
				defer wg.Wait()

				// ---- continuous read data (actually not reading, just wait for aborting)
				C_user_data := make(chan []byte, 5)
				ctx, stop := context.WithCancel(s.Ctx)
				channel.FromUser = C_user_data

				wg.Add(1) // handle data from user
				go func() {
					defer wg.Done()
					defer conn.Close() // if session end, or user close connection, close tcp connection

					stopped_by_ctx := utils.ReadChanUnderContext(ctx, C_user_data, func(data []byte) {
						// nothing to do -- http request is not duplex
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
					for h := range http_res.Header {
						dial_result.Headers = append(dial_result.Headers, biz.ProxyHttpHeader{
							Name:  h,
							Value: http_res.Header.Get(h),
						})
					}
					dial_result.StatusCode = int32(http_res.StatusCode)
					send_dial_result()

					// ---- continuous send to user
					for {
						data := make([]byte, 1024)
						n, err := conn.Read(data)
						if err != nil {
							return
						}

						if n > 0 {
							s.Write(utils.JoinBytes2(0x21, idBytes, data[:n]))
						}
					}
				}()
			}
		}()
	}
}
