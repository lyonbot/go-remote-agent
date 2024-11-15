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
	conn, err := ws.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	ws := utils.MakeRWChanFromWebSocket(conn)
	defer ws.Close()

	// make a tunnel
	agent_name := r.PathValue("agent_name") // required
	agent_id := r.FormValue("agent_id")     // optional
	tunnel, err := agent_handler.MakeAgentTunnel(agent_name, agent_id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer tunnel.Close()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ws.Close()
		for data := range tunnel.ChFromAgent {
			ws.Write(data)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer tunnel.Close()
		for data := range ws.Read {
			tunnel.ChToAgent <- data
		}
	}()

	// notify agent
	if err := tunnel.NotifyAgent(biz.AgentNotify{
		Type: "pty",
	}); err != nil {
		tunnel.Close()
		ws.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	wg.Wait()
}
