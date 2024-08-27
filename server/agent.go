package server

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net/http"
	"remote-agent/biz"
	"remote-agent/utils"
	"sync"
	"sync/atomic"
	"time"
)

var agents = sync.Map{}

type Agent struct {
	Name  string
	Chan  chan []byte
	Count atomic.Int64
}

func handleAgentTaskStreamRequest(w http.ResponseWriter, r *http.Request) {
	agent_name := r.PathValue("agent_name")
	if agent_name == "" {
		http.Error(w, "agent_name is required", http.StatusBadRequest)
		return
	}

	// sse stream
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// setup in agents
	agent := new(Agent)
	if agent_raw, is_new := agents.LoadOrStore(agent_name, agent); is_new {
		log.Printf("reuse agent: %s", agent_name)
		agent = agent_raw.(*Agent)
	} else {
		log.Printf("new agent: %s", agent_name)
		agent.Name = agent_name
		agent.Chan = make(chan []byte, 5)
	}

	agent.Count.Add(1)
	defer func() {
		remain := agent.Count.Add(-1)
		log.Printf("agent leave: %s, remain: %d", agent_name, remain)
		if remain == 0 {
			agents.Delete(agent_name)
			log.Printf("agent deleted: %s", agent_name)
		}
	}()

	// pipe stream to http
	write := func(data []byte) {
		binary.Write(w, binary.LittleEndian, uint32(len(data)))
		w.(io.Writer).Write(data)
		w.(io.Writer).Write([]byte("\r\n"))
		w.(http.Flusher).Flush()
	}

	alive_interval := time.NewTicker(time.Second * 30)
	defer alive_interval.Stop()

	ping := biz.AgentNotify{Type: "ping"}
	ping_data, _ := ping.MarshalMsg(nil)

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-agent.Chan:
			write(msg)
		case <-alive_interval.C:
			write(ping_data)
		}
	}
}

func handleAgentTaskWSRequest(w http.ResponseWriter, r *http.Request) {
	var agent *Agent
	if agent_raw, ok := agents.Load(r.PathValue("agent_name")); ok {
		agent = agent_raw.(*Agent)
	} else {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	var client *ClientTunnel
	if client_raw, ok := ClientTunnels.LoadAndDelete(r.PathValue("token")); ok {
		client = client_raw.(*ClientTunnel)
	} else {
		http.Error(w, "client not found", http.StatusNotFound)
		return
	}

	log.Printf("new ws connection: %s for %s", agent.Name, client.Token)

	// go websocket

	c, err := ws.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}

	defer c.Close()
	wg := sync.WaitGroup{}

	ch := utils.WSConnToChannels(c, &wg)

	wg.Add(2)

	// write stdin / signal
	go func() {
		defer wg.Done()
		for data := range client.Stdin {
			ch.Write <- bytes.Join([][]byte{[]byte{0x00}, data}, []byte{})
		}
		ch.Write <- []byte{0x01} // close stdin
	}()

	// read stdout / stderr / exit code etc.
	go func() {
		defer close(client.ExitCode)
		if client.Stdout != nil {
			defer close(*client.Stdout)
		}
		if client.Stderr != nil {
			defer close(*client.Stderr)
		}
		if client.RawFromAgent != nil {
			defer close(*client.RawFromAgent)
		}
		defer wg.Done()

		for data := range ch.Read {
			// clone raw data to client, if needed
			if client.RawFromAgent != nil {
				*client.RawFromAgent <- bytes.Clone(data)
			}

			t := data[0]

			if t == 0x00 {
				exit_code := int32(binary.LittleEndian.Uint32(data[1:]))
				client.ExitCode <- exit_code
				return
			}

			if t == 0x01 && client.Stdout != nil {
				*client.Stdout <- data[1:]
				continue
			}

			if t == 0x02 && client.Stderr != nil {
				*client.Stderr <- data[1:]
				continue
			}

			if t == 0x03 {
				log.Println("client:", agent.Name, "debug:", string(data[1:]))
				continue
			}
		}
	}()

	wg.Wait()
}
