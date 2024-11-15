package client_handler

import (
	"log"
	"net/http"
	"remote-agent/biz"
	"remote-agent/server/agent_handler"
	"remote-agent/utils"
	"sync"
)

func HandleClientPty(w http.ResponseWriter, r *http.Request) {
	if block_if_request_api_key_bad(w, r) {
		return
	}

	// websocket to client
	c, err := ws.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	wg := sync.WaitGroup{}
	ch := utils.MakeRWChanFromWebSocket(c, &wg)
	C_from_client := ch.Read

	// make a tunnel
	agent_name := r.PathValue("agent_name") // required
	agent_id := r.FormValue("agent_id")     // optional
	tunnel, _, _, notifyAgent, C_to_agent, C_from_agent, err := agent_handler.MakeAgentTunnel(agent_name, agent_id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer tunnel.Delete()

	// notify agent
	if err := notifyAgent(biz.AgentNotify{
		Type: "pty",
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// proxy agent's data
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ch.Close()
		defer c.Close()

		for data := range C_from_agent {
			ch.Write(data)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(C_to_agent)

		for data := range C_from_client {
			C_to_agent <- data
		}
	}()

	wg.Wait()
}
