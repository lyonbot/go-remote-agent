package server

import (
	"encoding/binary"
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

	RawFromAgent *chan []byte // cloned from agent raw ws data.
	Stdin        chan []byte  // for client to write
	Stdout       *chan []byte // for agent to write
	Stderr       *chan []byte // for agent to write
	ExitCode     chan int32   // for agent to write

	// to be filled
}

var ClientTunnels = sync.Map{} // map[string]*ClientTunnel

// make a empty client tunnel. you shall fill the content
func makeClientTunnel(r *http.Request) (tunnel *ClientTunnel, agent *Agent, err error) {
	agent_name := r.PathValue("agent_name")
	if agent_raw, ok := agents.Load(agent_name); ok {
		agent = agent_raw.(*Agent)
	} else {
		err = errors.New("agent not found")
		return
	}

	token := fmt.Sprintf("%x-%x", time.Now().Unix(), rand.Int31())
	tunnel = &ClientTunnel{
		Token:    token,
		Agent:    agent_name,
		Stdin:    make(chan []byte, 1024),
		ExitCode: make(chan int32, 1),
		// optional channels are created when needed
	}
	ClientTunnels.Store(token, tunnel)

	return
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

	tunnel, agent, err := makeClientTunnel(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if full {
		ch := make(chan []byte, 5)
		tunnel.RawFromAgent = &ch
	} else {
		if stdout {
			ch := make(chan []byte, 5)
			tunnel.Stdout = &ch
		}
		if stderr {
			ch := make(chan []byte, 5)
			tunnel.Stderr = &ch
		}
	}

	// handle stdin

	if file, headers, err := r.FormFile("stdin"); err == nil && headers != nil {
		stdin = true
		go utils.ReaderToChannel(tunnel.Stdin, file)
	} else if data := r.FormValue("stdin"); data != "" {
		stdin = true
		tunnel.Stdin <- []byte(data)
		close(tunnel.Stdin)
	} else {
		close(tunnel.Stdin)
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
	agent.Chan <- msg_data

	// make a chunked response

	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	writer := w.(io.Writer)

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for exit_code := range tunnel.ExitCode {
			// TODO: shall we print to client?
			log.Printf("client: %s exit code: %d", agent.Name, exit_code)
		}
	}()

	if tunnel.Stdout != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for data := range *tunnel.Stdout {
				writer.Write(data)
				w.(http.Flusher).Flush()
			}
		}()
	}

	if tunnel.Stderr != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for data := range *tunnel.Stderr {
				writer.Write(data)
				w.(http.Flusher).Flush()
			}
		}()
	}

	if tunnel.RawFromAgent != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for data := range *tunnel.RawFromAgent {
				binary.Write(writer, binary.LittleEndian, uint32(len(data)))
				writer.Write(data)
				w.(http.Flusher).Flush()
			}
		}()
	}

	wg.Wait()
}
