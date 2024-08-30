package client_handler

import (
	"encoding/json"
	"net/http"
	"remote-agent/server/agent_handler"
	"sync"
)

func write_agent_instance_list(w http.ResponseWriter, instances *sync.Map) {
	w.Write([]byte("["))
	is_first := true

	instances.Range(func(key, value interface{}) bool {
		instance := value.(*agent_handler.AgentInstance)
		b, err := json.Marshal(instance)
		if err != nil {
			return true
		}

		if !is_first {
			w.Write([]byte(","))
		}
		is_first = false

		w.Write(b)
		return true
	})
	w.Write([]byte("]"))
}

func HandleClientListAll(w http.ResponseWriter, r *http.Request) {
	if block_if_request_api_key_bad(w, r) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	write_agent_instance_list(w, &agent_handler.AllAgentInstances)
}

func HandleClientListAgent(w http.ResponseWriter, r *http.Request) {
	if block_if_request_api_key_bad(w, r) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	name := r.PathValue("agent_name")
	if raw, ok := agent_handler.Agents.Load(name); ok {
		agent := raw.(*agent_handler.Agent)
		write_agent_instance_list(w, &agent.Instances)
	} else {
		w.Write([]byte("[]"))
		return
	}
}
