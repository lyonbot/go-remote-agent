package client_handler

import (
	"net/http"
)

func HandleUpgradeRequest(w http.ResponseWriter, r *http.Request) {
	if block_if_request_api_key_bad(w, r) {
		return
	}

	// wg := sync.WaitGroup{}

	// agent_name := r.PathValue("agent_name") // required
	// agent_id := r.FormValue("agent_id")     // optional
	// tunnel, _, _, notifyAgent, C_to_agent, C_to_server, err := agent_handler.MakeAgentTunnel(agent_name, agent_id)
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }
	// defer agent_handler.DeleteAgentTunnel(tunnel)

	// // send msg to agent
	// notifyAgent(biz.AgentNotify{
	// 	Type: "upgrade",
	// })

}
