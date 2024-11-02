package agent_handler

import (
	"context"
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

func (agent *Agent) Delete() {
	Agents.Delete(agent.Name)
	close(agent.Channel)
}

var AllAgentInstances = sync.Map{}

type AgentInstance struct {
	Id           uint64          `json:"id"`
	Name         string          `json:"name"`
	UserAgent    string          `json:"user_agent"`
	IsUpgradable bool            `json:"is_upgradable"`
	JoinAt       time.Time       `json:"join_at"`
	RemoteAddr   string          `json:"remote_addr"`
	C            chan<- []byte   `json:"-"` // write task to this agent
	Ctx          context.Context `json:"-"` // if agent disconnected, this context will be Done
}

var AllProxyChannels = sync.Map{}

type ProxyChannel struct {
	Id  string
	Ctx context.Context
}
