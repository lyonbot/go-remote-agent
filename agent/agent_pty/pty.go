package agent_pty

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"remote-agent/biz"
	"remote-agent/utils"

	ptylib "github.com/creack/pty"
)

func (s *ptySessionRecv) setupPty() {
	var pty *os.File
	ws := s.ws

	// listener: pty data write
	s.handlers[0x00] = func(recv []byte) {
		if pty != nil {
			pty.Write(recv[1:])
		}
	}

	// listener: start pty
	s.handlers[0x01] = func(recv []byte) {
		if pty != nil {
			s.WriteDebugMessage("pty already opened")
		} else {
			req := biz.StartPtyRequest{}

			if len(recv[1:]) > 0 {
				if _, err := req.UnmarshalMsg(recv[1:]); err != nil {
					s.WriteDebugMessage(err.Error())
					return
				}
			}
			if req.Cmd == "" {
				req.Cmd = "sh"
			}

			c := exec.Command(req.Cmd, req.Args...)
			if req.InheritEnv {
				c.Env = append(os.Environ(), req.Env...)
			} else {
				c.Env = req.Env
			}

			var err error

			pty, err = ptylib.Start(c)
			if err != nil {
				s.WriteDebugMessage(err.Error())
				return
			}

			s.wg.Add(1)
			go func() {
				pty_closed := make(chan bool, 2)
				defer func() {
					pty_closed <- true
				}()

				s.wg.Add(1)
				go func() {
					select {
					case <-pty_closed: // pty closed
					case <-s.ctx.Done(): // session end
					}

					pty.Close()
					pty = nil
					utils.TryWrite(ws.Write, []byte{0x02}) // pty closed
					s.wg.Done()
				}()

				for {
					data := make([]byte, 1024)
					n, err := pty.Read(data)
					if err != nil {
						s.WriteDebugMessage(err.Error())
						return
					}

					utils.TryWrite(ws.Write, utils.PrependBytes([]byte{0x00}, data[:n]))
				}
			}()

			ws.Write <- []byte{0x01} // pty opened
		}
	}

	// listener: close pty
	s.handlers[0x02] = func(recv []byte) {
		if pty != nil {
			if err := pty.Close(); err != nil {
				s.WriteDebugMessage(err.Error())
			}
		}
	}

	// listener: resize pty
	s.handlers[0x03] = func(recv []byte) {
		if pty != nil {
			cols := uint16(binary.LittleEndian.Uint16(recv[1:]))
			rows := uint16(binary.LittleEndian.Uint16(recv[3:]))
			width := uint16(binary.LittleEndian.Uint16(recv[5:]))
			height := uint16(binary.LittleEndian.Uint16(recv[7:]))
			size := ptylib.Winsize{
				Cols: cols,
				Rows: rows,
				X:    width,
				Y:    height,
			}
			if err := ptylib.Setsize(pty, &size); err != nil {
				s.WriteDebugMessage(fmt.Sprintf("pty resize failed: %s", err.Error()))
			}
		}
	}
}
