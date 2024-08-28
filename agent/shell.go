package agent

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"remote-agent/biz"
	"remote-agent/utils"
	"strings"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
)

func run_shell(task *biz.AgentNotify) {
	url := biz.Config.BaseUrl + "/api/agent/" + biz.Config.Name + "/" + task.Id
	url = strings.Replace(url, "http", "ws", 1)

	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return
	}
	defer c.Close()

	wg := sync.WaitGroup{}
	ws := utils.WSConnToChannels(c, &wg)

	// ---- setup process

	cmd := exec.Command("sh", "-c", task.Cmd)
	code := int32(-1)

	print_error_message := func(msg string) {
		bytes := []byte(msg)
		// concat with 0x03
		data := make([]byte, len(bytes)+1)
		data[0] = 0x03
		copy(data[1:], bytes)

		log.Println("shell:", msg)
		ws.Write <- data
	}

	// pipes

	stdin, err0 := cmd.StdinPipe()
	stdout, err1 := cmd.StdoutPipe()
	stderr, err2 := cmd.StderrPipe()

	if err0 != nil {
		print_error_message(fmt.Sprintln("failed to get stdin pipe:", err))
		return
	}
	if err1 != nil {
		print_error_message(fmt.Sprintln("failed to get stdout pipe:", err1))
		return
	}
	if err2 != nil {
		print_error_message(fmt.Sprintln("failed to get stderr pipe:", err2))
		return
	}

	// -- setup stdout/stderr

	pipeOutputToUpstream := func(prefix byte, enabled bool, r io.ReadCloser) {
		defer wg.Done()

		defer r.Close()

		for {
			data := make([]byte, 1025)
			data[0] = prefix
			n, err := r.Read(data[1:])
			if err != nil {
				break
			}

			// discard if not enabled
			if enabled {
				ws.Write <- data[:n+1]
			}
		}
	}

	wg.Add(2)
	go pipeOutputToUpstream(0x01, task.NeedStdout, stdout)
	go pipeOutputToUpstream(0x02, task.NeedStderr, stderr)

	// -- start

	if err := cmd.Start(); err != nil {
		print_error_message(fmt.Sprintln("failed to start command:", err))
		return
	}
	log.Println("start command:", task.Cmd, "pid:", cmd.Process.Pid)

	// handle data from client

	wg.Add(1)
	go func() {
		defer wg.Done()

		// note: remote will close stdin.
		// has_stdin := task.HasStdin
		// if !has_stdin {
		// 	stdin.Close()
		// }

		for data := range ws.Read {
			t := data[0]

			// write stdin
			if t == 0x00 {
				if _, err := stdin.Write(data[1:]); err != nil {
					print_error_message(fmt.Sprintln("failed to write stdin:", err))
				}
				continue
			}

			// close stdin
			if t == 0x01 {
				if err := stdin.Close(); err != nil {
					print_error_message(fmt.Sprintln("failed to close stdin:", err))
				}
				continue
			}

			// send signal
			if t == 0x02 {
				var signal os.Signal
				var st int32 = int32(binary.LittleEndian.Uint32(data[1:]))

				switch st {
				case 0x2:
					signal = os.Interrupt
				case 0x9:
					signal = os.Kill
				case 0x1e:
					signal = syscall.SIGUSR1
				case 0x1f:
					signal = syscall.SIGUSR2
				default:
					print_error_message(fmt.Sprintln("unknown signal:", st))
					continue
				}

				if err := cmd.Process.Signal(signal); err != nil {
					print_error_message(fmt.Sprintln("failed to send signal:", err))
				}
				continue
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			code = int32(exiterr.ExitCode())
		} else {
			print_error_message(fmt.Sprintln("failed to wait command:", err))
		}
	} else {
		code = 0
	}

	// -- end

	log.Println("exit code:", code)

	data := []byte{0x00, 0xff, 0xff, 0xff, 0xff}
	binary.LittleEndian.PutUint32(data[1:], uint32(code))
	ws.Write <- data
	close(ws.Write)

	wg.Wait()
}
