package agent_omni

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"remote-agent/utils"
	"sync"
)

func (s *PtySession) SetupTcpProxy() {
	type ProxyChannel struct {
		c     io.ReadWriteCloser
		close func()
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

		if _, exists := channels.Swap(id, struct{}{}); exists {
			s.Write(utils.JoinBytes2(0x20, idBytes, []byte{0x01}))
			s.WriteDebugMessage(fmt.Sprintf("tcp proxy 0x%x already opened", id))
			return
		}

		go func() {
			var c io.ReadWriteCloser

			if true {
				// tcp proxy
				conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", addr, port))
				if err != nil {
					s.Write(utils.JoinBytes2(
						0x20,
						idBytes,
						[]byte{0x01},
						[]byte(err.Error()),
					))
					return
				}

				defer conn.Close()
				s.Write(utils.JoinBytes2(
					0x20,
					idBytes,
					[]byte{0x00},
					[]byte(conn.LocalAddr().String()),
				))

				c = conn.(io.ReadWriteCloser)
			}

			// ---- continuous read data
			ctx, close := context.WithCancel(context.Background())
			channel := &ProxyChannel{
				c:     c,
				close: close,
			}
			channels.Store(id, channel)

			go func() {
				select {
				case <-ctx.Done():
				case <-s.Ctx.Done():
				}

				c.Close()
				channels.Delete(id)
				s.Write(utils.JoinBytes2(0x22, idBytes))
			}()

			for {
				data := make([]byte, 1024)
				n, err := c.Read(data)
				if err != nil {
					// s.WriteDebugMessage(fmt.Sprintf("tcp proxy 0x%x read error: %s", id, err.Error()))
					close()
					return
				}

				s.Write(utils.JoinBytes2(0x21, idBytes, data[:n]))
			}
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

		channel, ok := channels.Load(id)
		if !ok {
			s.WriteDebugMessage(fmt.Sprintf("tcp proxy 0x%x not found", id))
			return
		}
		channel.(*ProxyChannel).c.Write(recv[5:])
	}

	// close tcp proxy channel
	s.Handlers[0x22] = func(recv []byte) {
		if len(recv) < 5 {
			s.WriteDebugMessage("invalid tcp proxy channel request")
			return
		}

		idBytes := recv[1:5]
		id := binary.LittleEndian.Uint32(idBytes)

		channel, ok := channels.Load(id)
		if !ok {
			s.WriteDebugMessage(fmt.Sprintf("tcp proxy 0x%x not found", id))
			return
		}
		channel.(*ProxyChannel).close()
	}
}
