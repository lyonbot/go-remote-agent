package agent

import (
	"os"
	"os/exec"
	"remote-agent/biz"
	"remote-agent/utils"
	"strings"
	"sync"

	ptylib "github.com/creack/pty"
	"github.com/gorilla/websocket"
)

func run_pty(task *biz.AgentNotify) {
	url := biz.Config.BaseUrl + "/api/agent/" + biz.Config.Name + "/" + task.Id
	url = strings.Replace(url, "http", "ws", 1)
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return
	}
	defer c.Close()

	wg := sync.WaitGroup{}
	ws := utils.WSConnToChannels(c, &wg)
	defer close(ws.Write)

	write_debug_message := func(msg string) {
		ws.Write <- utils.PrependBytes([]byte{0x03}, []byte(msg))
	}

	var pty *os.File
	defer func() {
		if pty != nil {
			pty.Close()
			pty = nil
		}
	}()

	for recv := range ws.Read {
		t := recv[0]

		switch t {
		case 0x00:
			if pty != nil {
				pty.Write(recv[1:])
			}
			continue

		case 0x01:
			if pty != nil {
				write_debug_message("pty already opened")
			} else {
				cmd_str := strings.Split(string(recv[1:]), "\x00")
				c := exec.Command(cmd_str[0], cmd_str[1:]...)
				c.Env = append(c.Env, "TERM=xterm-256color")
				new_pty, err := ptylib.Start(c)
				if err != nil {
					write_debug_message(err.Error())
				} else {
					pty = new_pty

					wg.Add(1)
					go func() {
						defer wg.Done()
						defer func() {
							pty.Close()
							pty = nil
							ws.Write <- []byte{0x02} // pty closed
						}()

						for {
							data := make([]byte, 1024)
							n, err := pty.Read(data)
							if err != nil {
								write_debug_message(err.Error())
								return
							}

							ws.Write <- utils.PrependBytes([]byte{0x00}, data[:n])
						}
					}()
					ws.Write <- []byte{0x01} // pty opened
				}
			}
			continue

		case 0x02:
			if pty != nil {
				if err := pty.Close(); err != nil {
					write_debug_message(err.Error())
				}
			}
			continue
		}
	}
}
