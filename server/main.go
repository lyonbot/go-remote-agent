package server

import (
	"fmt"
	"log"
	"net/http"
	"remote-agent/biz"
	"remote-agent/server/agent_handler"
	"remote-agent/server/assets"
	"remote-agent/server/client_handler"
)

func RunServer() {
	addr := fmt.Sprintf("%s:%d", biz.Config.Addr, biz.Config.Port)

	http.HandleFunc("/api/agent/{agent_name}", agent_handler.HandleTaskStreamRequest)
	http.HandleFunc("/api/agent/{agent_name}/{token}", agent_handler.HandleAgentTunnelRequest)

	http.HandleFunc("/api/client/", client_handler.HandleClientListAll)
	http.HandleFunc("/api/client/{agent_name}/", client_handler.HandleClientListAgent)
	http.HandleFunc("/api/client/{agent_name}/exec/", client_handler.HandleClientExec)
	http.HandleFunc("/api/client/{agent_name}/pty/", client_handler.HandleClientPty)
	http.HandleFunc("/api/client/{agent_name}/upgrade/", client_handler.HandleUpgradeRequest)

	http.HandleFunc("/", assets.HandleWebAssets)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalln("failed to ListenAndServe:", err)
		panic(err)
	}

	log.Println("Listening on", addr)
}
