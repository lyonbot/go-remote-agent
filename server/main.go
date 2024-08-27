package server

import (
	"fmt"
	"log"
	"net/http"
	"remote-agent/biz"

	"github.com/gorilla/websocket"
)

var ws = websocket.Upgrader{} // use default options

func RunServer() {
	addr := fmt.Sprintf("%s:%d", biz.Config.Addr, biz.Config.Port)

	http.HandleFunc("/api/agent/{agent_name}", handleAgentTaskStreamRequest)
	http.HandleFunc("/api/agent/{agent_name}/{token}", handleAgentTaskWSRequest)
	http.HandleFunc("/api/client/{agent_name}/exec/", handleClientExec)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalln("failed to ListenAndServe:", err)
		panic(err)
	}

	log.Println("Listening on", addr)
}
