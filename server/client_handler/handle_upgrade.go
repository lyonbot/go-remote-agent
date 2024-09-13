package client_handler

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"remote-agent/biz"
	"remote-agent/server/agent_handler"
)

func HandleUpgradeRequest(w http.ResponseWriter, r *http.Request) {
	if block_if_request_api_key_bad(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// make a tunnel
	agent_name := r.PathValue("agent_name") // required
	agent_id := r.FormValue("agent_id")     // required

	tunnel, _, agent_instance, notifyAgent, C_to_agent, C_from_agent, err := agent_handler.MakeAgentTunnel(agent_name, agent_id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer tunnel.Delete()

	if agent_instance == nil {
		http.Error(w, "agent not found. make sure agent_id is correct", http.StatusBadRequest)
		return
	}

	// make a chunked response

	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	writer := w.(io.Writer)
	write_to_http := func(data string) {
		writer.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
		w.(http.Flusher).Flush()
	}

	// check if upgradable
	write_to_http("agent_info: " + fmt.Sprintf("%+v", agent_instance))
	if !agent_instance.IsUpgradable {
		write_to_http("error: agent is not upgradable")
		return
	}

	// get self executable file
	executable_binary, err := getSelfExecutableFile()
	if err != nil {
		write_to_http(fmt.Sprintf("error: cannot get self executable file: %s", err.Error()))
		return
	}

	// send msg to agent
	notifyAgent(biz.AgentNotify{
		Type: "upgrade",
	})

	var recv []byte
	var ok bool
	tryRecv := func(min_len int, prefix byte, stage string) (success bool) {
		recv, ok = <-C_from_agent

		if !ok || recv == nil {
			write_to_http(fmt.Sprintf("error: %s, got nothing", stage))
			return false
		}

		if recv[0] == 0x99 {
			write_to_http(fmt.Sprintf("error: %s, got error: %s", stage, string(recv[1:])))
			return false
		}

		if len(recv) < min_len || recv[0] != prefix {
			write_to_http(fmt.Sprintf("error: %s, got bad package", stage))
			return false
		}

		return true
	}

	// wait for ready
	C_to_agent <- []byte{0x00}
	if !tryRecv(1, 0x00, "wait for ready") {
		return
	}
	write_to_http(fmt.Sprintf("remote executable path: %s", string(recv[1:])))

	// send file size
	total_size := len(executable_binary)
	C_to_agent <- binary.LittleEndian.AppendUint64([]byte{0x01}, uint64(total_size))
	write_to_http("start send chunks")

	// send chunks
	chunk_size := 1024 * 1024
	current_offset := 0
	for {
		if current_offset >= total_size {
			break
		}

		end := current_offset + chunk_size
		if end > total_size {
			end = total_size
		}

		chunk := executable_binary[current_offset:end]
		actual_size := len(chunk)

		to_send := make([]byte, 9+actual_size)
		to_send[0] = 0x02
		binary.LittleEndian.PutUint64(to_send[1:9], uint64(current_offset))
		copy(to_send[9:], chunk)

		// send chunk
		current_offset += actual_size
		C_to_agent <- to_send

		// check if write ok
		if !tryRecv(9, 0x00, "remote write chunk") {
			return
		}
		new_offset := int(binary.LittleEndian.Uint64(recv[1:9]))
		if new_offset != current_offset {
			write_to_http(fmt.Sprintf("error: remote cannot write executable file, offset mismatch: %d != %d", new_offset, current_offset))
			return
		}

		write_to_http(fmt.Sprintf("remote wrote chunk: %d, percentage: %.2f%%", new_offset, float64(new_offset)/float64(len(executable_binary))*100))
	}

	// all sent
	if !tryRecv(1, 0x01, "recv done") {
		return
	}
	write_to_http("remote finish recv")

	// started new executable
	if !tryRecv(1, 0x02, "remote started new executable") {
		return
	}
	write_to_http("remote started new executable")
}

func getSelfExecutableFile() ([]byte, error) {
	exec_path, err := os.Executable()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(exec_path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	data := make([]byte, stat.Size())
	_, err = file.Read(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}
