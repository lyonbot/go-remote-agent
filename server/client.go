package server

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"remote-agent/biz"
	"remote-agent/utils"
	"sync"
	"time"
)

type ClientTunnel struct {
	Token string
	Agent string

	ToAgent  <-chan []byte
	ToServer chan<- []byte
}

var ClientTunnels = sync.Map{} // map[string]*ClientTunnel

// make a empty client tunnel. you shall fill the content
func makeClientTunnel(r *http.Request) (tunnel *ClientTunnel, agent *Agent, C_to_agent chan<- []byte, C_to_server <-chan []byte, err error) {
	agent_name := r.PathValue("agent_name")
	if agent_raw, ok := agents.Load(agent_name); ok {
		agent = agent_raw.(*Agent)
	} else {
		err = errors.New("agent not found")
		return
	}

	to_agent := make(chan []byte, 5)
	to_server := make(chan []byte, 5)

	C_to_agent = (chan<- []byte)(to_agent)
	C_to_server = (<-chan []byte)(to_server)

	token := fmt.Sprintf("%x-%x", time.Now().Unix(), rand.Int31())
	tunnel = &ClientTunnel{
		Token:    token,
		Agent:    agent_name,
		ToAgent:  to_agent,
		ToServer: to_server,
	}
	ClientTunnels.Store(token, tunnel)

	return
}

func handleClientListAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	write_agent_instance_list(w, &all_agent_instances)
}

func handleClientListAgent(w http.ResponseWriter, r *http.Request) {
	var agent *Agent
	if raw, ok := agents.Load(r.PathValue("agent_name")); ok {
		agent = raw.(*Agent)
	} else {
		http.Error(w, "invalid agent name", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	write_agent_instance_list(w, &agent.Instances)
}

func write_agent_instance_list(w http.ResponseWriter, instances *sync.Map) {
	w.Write([]byte("["))
	is_first := true

	instances.Range(func(key, value interface{}) bool {
		instance := value.(*AgentInstance)
		b, err := json.Marshal(instance)
		if err != nil {
			return true
		}

		if !is_first {
			w.Write([]byte(","))
		}
		is_first = false

		w.Write(b)
		return true
	})
	w.Write([]byte("]"))
}

func handleClientExec(w http.ResponseWriter, r *http.Request) {
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
	tunnel, agent, C_to_agent, C_to_server, err := makeClientTunnel(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

	msg := biz.AgentNotify{
		Type:       "shell",
		Id:         tunnel.Token,
		Cmd:        cmd,
		HasStdin:   stdin,
		NeedStdout: stdout || full,
		NeedStderr: stderr || full,
	}
	msg_data, _ := msg.MarshalMsg(nil)
	agent.Channel <- msg_data

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
				log.Printf("client: %s exit code: %d", agent.Name, exit_code)

			case 0x01:
				if !full && stdout {
					write_to_http(data[1:])
				}

			case 0x02:
				if !full && stderr {
					write_to_http(data[1:])
				}

			case 0x03:
				log.Println("client:", agent.Name, "debug:", string(data[1:]))
			}
		}
	}()

	wg.Wait()
}

func handleClientPty(w http.ResponseWriter, r *http.Request) {
	// websocket to client
	c, err := ws.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	wg := sync.WaitGroup{}
	ch := utils.WSConnToChannels(c, &wg)
	C_from_client := ch.Read
	C_to_client := ch.Write

	// make a tunnel
	tunnel, agent, C_to_agent, C_from_agent, err := makeClientTunnel(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// notify agent
	msg := biz.AgentNotify{
		Type: "pty",
		Id:   tunnel.Token,
	}
	msg_data, _ := msg.MarshalMsg(nil)
	agent.Channel <- msg_data

	// proxy agent's data
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(C_to_client)
		defer c.Close()

		for data := range C_from_agent {
			C_to_client <- data
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(C_to_agent)

		for data := range C_from_client {
			C_to_agent <- data
		}
	}()

	wg.Wait()
}
