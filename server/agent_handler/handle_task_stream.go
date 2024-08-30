package agent_handler

import (
	"encoding/binary"
	"io"
	"log"
	"net/http"
	"remote-agent/biz"
	"sync/atomic"
	"time"
)

var agent_instance_id_counter = atomic.Uint64{}

func HandleTaskStreamRequest(w http.ResponseWriter, r *http.Request) {
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
	if agent_raw, ok := Agents.LoadOrStore(agent_name, agent); ok {
		log.Printf("reuse agent: %s", agent_name)
		agent = agent_raw.(*Agent)
	} else {
		log.Printf("new agent: %s", agent_name)
		agent.Name = agent_name
		agent.Channel = make(chan []byte, 5)
	}

	agent.Count.Add(1)
	defer func() {
		remain := agent.Count.Add(-1)
		log.Printf("agent leave: %s, remain: %d", agent_name, remain)
		if remain == 0 {
			agent.Delete()
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

	// add agent instance

	instance_id := agent_instance_id_counter.Add(1)
	user_agent := r.Header.Get("User-Agent")
	instance_chan := make(chan []byte, 5)
	instance := AgentInstance{
		Id:            instance_id,
		Name:          agent_name,
		UserAgent:     user_agent,
		IsUpgradable:  biz.IsUserAgentCanBeUpgraded(user_agent),
		JoinAt:        time.Now(),
		RemoteAddr:    r.RemoteAddr,
		NotifyChannel: instance_chan,
	}

	AllAgentInstances.Store(instance_id, &instance)
	defer AllAgentInstances.Delete(instance_id)
	agent.Instances.Store(instance_id, &instance)
	defer agent.Instances.Delete(instance_id)
	defer close(instance_chan)

	// keep alive interval

	alive_interval := time.NewTicker(time.Second * 30)
	defer alive_interval.Stop()
	ping := biz.AgentNotify{Type: "ping"}
	ping_data, _ := ping.MarshalMsg(nil)

loop:
	for {
		select {
		case <-r.Context().Done():
			break loop
		case msg := <-agent.Channel:
			write(msg)
		case msg := <-instance_chan:
			write(msg)
		case <-alive_interval.C:
			write(ping_data)
		}
	}
}
