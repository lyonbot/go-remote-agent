package agent_handler

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"remote-agent/biz"
	"remote-agent/utils"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// a AgentTunnel is a bidirectional channel between agent and server
//
// use MakeAgentTunnel to make one
type AgentTunnel struct {
	Token string

	Agent         *Agent
	AgentInstance *AgentInstance                     // Note: can be nil, if agentId not specified
	NotifyAgent   func(notify biz.AgentNotify) error // when listeners are set, notify agent to start a new session. can only call once

	ChToAgent   chan<- []byte // data send to agent -- do not close()
	ChFromAgent <-chan []byte

	pipeToWebSocketAndRun func(*websocket.Conn) // internal - start a loop, forwarding data via websocket. once finished, close websocket
	closeWs               context.CancelFunc
}

var AgentTunnels = sync.Map{} // map[string]*AgentTunnel

// make a empty agent tunnel. Usage:
//
// 1. make a tunnel
// 2. setup listeners on tunnel's ChToAgent and ChFromAgent
// 3. call tunnel.NotifyAgent to notify agent to start a new session
// 4. run your logic. call tunnel.Close() when done
//
// Note: remember to `defer tunnel.Delete()`
func MakeAgentTunnel(agent_name, agent_id string) (tunnel *AgentTunnel, err error) {
	var agent *Agent
	var agentInstance *AgentInstance // may be nil

	// ---- make notify fn

	var C_notify_agent chan<- []byte
	if agent_raw, ok := Agents.Load(agent_name); ok {
		agent = agent_raw.(*Agent)
		if agent_id == "" {
			C_notify_agent = agent.Channel
		} else if id_num, err := strconv.ParseUint(agent_id, 10, 64); err == nil {
			if instance, ok := agent.Instances.Load(id_num); ok {
				agentInstance = instance.(*AgentInstance)
				C_notify_agent = agentInstance.C
			}
		}
	}

	if C_notify_agent == nil {
		err = errors.New("agent not found")
		return
	}

	notified := false
	notifyAgent := func(notify biz.AgentNotify) error {
		if notified {
			return errors.New("notify agent only once")
		}
		notified = true
		notify.Id = tunnel.Token
		if msg_data, err := notify.MarshalMsg(nil); err != nil {
			return err
		} else {
			C_notify_agent <- msg_data
		}
		return nil
	}

	// ---- make channels

	token := fmt.Sprintf("%x-%x", time.Now().Unix(), rand.Int31())
	chToAgent := make(chan []byte)
	chFromAgent := make(chan []byte)
	pipeToWebSocketAndRun := func(conn *websocket.Conn) {
		AgentTunnels.Delete(token)

		wg := sync.WaitGroup{}
		ch := utils.MakeRWChanFromWebSocket(conn)

		tunnel.closeWs = ch.Close

		wg.Add(1)
		go func() {
			defer wg.Done()
			for data := range chToAgent {
				ch.Write(data)
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for data := range ch.Read {
				chFromAgent <- data
			}
			close(chFromAgent)
		}()

		wg.Wait()
	}

	tunnel = &AgentTunnel{
		Token: token,

		Agent:         agent,
		AgentInstance: agentInstance,
		NotifyAgent:   notifyAgent,

		ChToAgent:   chToAgent,
		ChFromAgent: chFromAgent,

		pipeToWebSocketAndRun: pipeToWebSocketAndRun,
	}
	AgentTunnels.Store(token, tunnel)

	return
}

func (tunnel *AgentTunnel) Close() {
	AgentTunnels.Delete(tunnel.Token)
	if tunnel.closeWs != nil {
		tunnel.closeWs()
	}
}
