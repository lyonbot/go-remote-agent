package agent_handler

import (
	"errors"
	"fmt"
	"math/rand"
	"remote-agent/biz"
	"strconv"
	"sync"
	"time"
)

// a AgentTunnel is a bidirectional channel between agent and server
//
// use MakeAgentTunnel to make one
type AgentTunnel struct {
	Token string
	Agent string

	ToAgent  <-chan []byte
	ToServer chan<- []byte
}

var AgentTunnels = sync.Map{} // map[string]*AgentTunnel

// make a empty agent tunnel. then you can notify agent via notifyAgent
//
// Note: agent_instance is nil if agent_id is empty
//
// Note: remember to `defer tunnel.Delete()`
func MakeAgentTunnel(agent_name, agent_id string) (tunnel *AgentTunnel, agent *Agent, agent_instance *AgentInstance, notifyAgent func(notify biz.AgentNotify) error, C_to_agent chan<- []byte, C_to_server <-chan []byte, err error) {
	var C_notify_agent chan<- []byte
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

	notifyAgent = func(notify biz.AgentNotify) error {
		notify.Id = tunnel.Token
		if msg_data, err := notify.MarshalMsg(nil); err != nil {
			return err
		} else {
			C_notify_agent <- msg_data
		}
		return nil
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

func (tunnel *AgentTunnel) Delete() {
	AgentTunnels.Delete(tunnel.Token)
}
