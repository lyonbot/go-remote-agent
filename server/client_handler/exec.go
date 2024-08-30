package client_handler

import (
	"encoding/binary"
	"io"
	"log"
	"net/http"
	"remote-agent/biz"
	"remote-agent/server/agent_handler"
	"remote-agent/utils"
	"sync"
)

func HandleClientExec(w http.ResponseWriter, r *http.Request) {
	if block_if_request_api_key_bad(w, r) {
		return
	}

	// parse request
	cmd := r.FormValue("cmd")
	stdout := utils.Defaults(r.FormValue("stdout"), "1") == "1"
	stderr := utils.Defaults(r.FormValue("stderr"), "0") == "1"
	full := r.FormValue("full") == "1"
	stdin := false // stdin is handled later

	if cmd == "" {
		http.Error(w, "cmd is required", http.StatusBadRequest)
		return
	}

	// make a tunnel

	wg := sync.WaitGroup{}

	agent_name := r.PathValue("agent_name") // required
	agent_id := r.FormValue("agent_id")     // optional
	tunnel, _, _, notifyAgent, C_to_agent, C_to_server, err := agent_handler.MakeAgentTunnel(agent_name, agent_id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer tunnel.Delete()

	{ // handle stdin
		C_stdin := make(chan []byte, 5)

		wg.Add(1)
		go func() {
			defer wg.Done()
			// no need to close -- agent will close it.

			for data := range C_stdin {
				C_to_agent <- utils.PrependBytes([]byte{0x00}, data)
			}
			C_to_agent <- []byte{0x01}
		}()

		if file, headers, err := r.FormFile("stdin"); err == nil && headers != nil {
			stdin = true
			go utils.ReaderToChannel(C_stdin, file)
		} else if data := r.FormValue("stdin"); data != "" {
			stdin = true
			C_stdin <- []byte(data)
			go close(C_stdin)
		} else {
			close(C_stdin)
		}
	}

	// send msg to agent

	if err := notifyAgent(biz.AgentNotify{
		Type:       "shell",
		Cmd:        cmd,
		HasStdin:   stdin,
		NeedStdout: stdout || full,
		NeedStderr: stderr || full,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// make a chunked response

	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Accel-Buffering", "no")
	// w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	writer := w.(io.Writer)
	write_to_http := func(data []byte) {
		writer.Write(data)
		w.(http.Flusher).Flush()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		for data := range C_to_server {
			if full {
				binary.Write(writer, binary.LittleEndian, uint32(len(data)))
				write_to_http(data)
			}

			switch data[0] {
			case 0x00:
				exit_code := int32(binary.LittleEndian.Uint32(data[1:]))
				log.Printf("%s exit code: %d", agent_name, exit_code)

			case 0x01:
				if !full && stdout {
					write_to_http(data[1:])
				}

			case 0x02:
				if !full && stderr {
					write_to_http(data[1:])
				}

			case 0x03:
				log.Println("client:", agent_id, "debug:", string(data[1:]))
			}
		}
	}()

	wg.Wait()
}
