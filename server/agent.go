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
	Name      string
	Channel   chan []byte  // write to arbitrary agent
	Count     atomic.Int64 // count of agent instances
	Instances sync.Map     // map[instance_id]*AgentInstance -- storing agent info and notify channel
}

var all_agent_instances = sync.Map{}
var agent_instance_id_counter = atomic.Uint64{}

type AgentInstance struct {
	Id            uint64        `json:"id"`
	Name          string        `json:"name"`
	UserAgent     string        `json:"user_agent"`
	IsUpgradable  bool          `json:"is_upgradable"`
	JoinAt        time.Time     `json:"join_at"`
	RemoteAddr    string        `json:"remote_addr"`
	NotifyChannel chan<- []byte `json:"-"`
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
	if agent_raw, ok := agents.LoadOrStore(agent_name, agent); ok {
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

	all_agent_instances.Store(instance_id, &instance)
	defer all_agent_instances.Delete(instance_id)
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
