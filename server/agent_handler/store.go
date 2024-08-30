package agent_handler

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var Agents = sync.Map{}

type Agent struct {
	Name      string
	Channel   chan []byte  // write to arbitrary agent
	Count     atomic.Int64 // count of agent instances
	Instances sync.Map     // map[instance_id]*AgentInstance -- storing agent info and notify channel
}

var AllAgentInstances = sync.Map{}
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

type AgentTunnel struct {
	Token string
	Agent string

	ToAgent  <-chan []byte
	ToServer chan<- []byte
}

var AgentTunnels = sync.Map{} // map[string]*AgentTunnel

// make a empty client tunnel. you shall fill the content
//
// Note: agent_instance is nil if agent_id is empty
func MakeAgentTunnel(r *http.Request) (tunnel *AgentTunnel, agent *Agent, agent_instance *AgentInstance, C_notify_agent chan<- []byte, C_to_agent chan<- []byte, C_to_server <-chan []byte, err error) {
	agent_id := r.FormValue("agent_id")     // optional
	agent_name := r.PathValue("agent_name") // required

	if agent_raw, ok := Agents.Load(agent_name); ok {
		agent = agent_raw.(*Agent)
		if agent_id == "" {
			C_notify_agent = agent.Channel
		} else if id_num, err := strconv.ParseUint(agent_id, 10, 64); err == nil {
			if instance, ok := agent.Instances.Load(id_num); ok {
				agent_instance = instance.(*AgentInstance)
				C_notify_agent = agent_instance.NotifyChannel
			}
		}
	}

	if C_notify_agent == nil {
		err = errors.New("agent not found")
		return
	}

	to_agent := make(chan []byte, 5)
	to_server := make(chan []byte, 5)

	C_to_agent = (chan<- []byte)(to_agent)
	C_to_server = (<-chan []byte)(to_server)

	token := fmt.Sprintf("%x-%x", time.Now().Unix(), rand.Int31())
	tunnel = &AgentTunnel{
		Token:    token,
		Agent:    agent_name,
		ToAgent:  to_agent,
		ToServer: to_server,
	}
	AgentTunnels.Store(token, tunnel)

	return
}
