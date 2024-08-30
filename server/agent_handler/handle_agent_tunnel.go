package agent_handler

import (
	"log"
	"net/http"
	"remote-agent/utils"
	"sync"

	"github.com/gorilla/websocket"
)

var ws = websocket.Upgrader{
	EnableCompression: true,
	CheckOrigin: func(r *http.Request) bool {
		// accept all origin -- be good with reverse proxies
		return true
	},
}

func HandleAgentTunnelRequest(w http.ResponseWriter, r *http.Request) {
	var agent *Agent
	if agent_raw, ok := Agents.Load(r.PathValue("agent_name")); ok {
		agent = agent_raw.(*Agent)
	} else {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	var tunnel *AgentTunnel
	if tunnel_raw, ok := AgentTunnels.LoadAndDelete(r.PathValue("token")); ok {
		tunnel = tunnel_raw.(*AgentTunnel)
	} else {
		http.Error(w, "tunnel not found", http.StatusNotFound)
		return
	}

	log.Printf("new ws connection: %s for %s", agent.Name, tunnel.Token)

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
			case data, ok := <-tunnel.ToAgent:
				if !ok {
					// no more data to agent, close connection
					return
				}
				ch.Write <- data
			case <-C_closed_from_agent:
				// WARNING: this may cause tunnel.ToAgent not closed?
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer close(tunnel.ToServer)

		for data := range ch.Read {
			tunnel.ToServer <- data
		}

		C_closed_from_agent <- struct{}{}
		close(C_closed_from_agent)
	}()

	wg.Wait()
}
