package server

import (
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
	C_closed_from_agent := make(chan struct{}, 1)

	wg.Add(2)

	go func() {
		defer wg.Done()
		defer c.Close()
		defer close(ch.Write)

		for {
			select {
			case data, ok := <-client.ToAgent:
				if !ok {
					// no more data to agent, close connection
					return
				}
				ch.Write <- data
			case <-C_closed_from_agent:
				// WARNING: this may cause client.ToAgent not closed?
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer close(client.ToServer)

		for data := range ch.Read {
			client.ToServer <- data
		}

		C_closed_from_agent <- struct{}{}
		close(C_closed_from_agent)
	}()

	wg.Wait()
}
