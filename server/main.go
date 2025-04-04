package server

import (
	"fmt"
	"log"
	"net/http"
	"remote-agent/biz"
	"remote-agent/server/agent_handler"
	"remote-agent/server/assets"
	"remote-agent/server/client_handler"
	"remote-agent/server/proxy"
	"strings"
)

func RunServer() {
	addr := fmt.Sprintf("%s:%d", biz.Config.Addr, biz.Config.Port)

	mux_agent := http.NewServeMux()
	mux_agent.HandleFunc("/api/agent/{agent_name}", agent_handler.HandleTaskStreamRequest)
	mux_agent.HandleFunc("/api/agent/{agent_name}/{token}", agent_handler.HandleAgentTunnelRequest)

	mux_client := http.NewServeMux()
	mux_client.HandleFunc("/api/agent/", client_handler.HandleClientListAll)
	mux_client.HandleFunc("/api/agent/{agent_name}/", client_handler.HandleClientListAgent)
	mux_client.HandleFunc("/api/agent/{agent_name}/exec/", client_handler.HandleClientExec)
	mux_client.HandleFunc("/api/agent/{agent_name}/omni/", client_handler.HandleClientPty)
	mux_client.HandleFunc("/api/agent/{agent_name}/upgrade/", client_handler.HandleUpgradeRequest)
	mux_client.HandleFunc("/api/proxy/", client_handler.HandleProxyListAll)
	mux_client.HandleFunc("/api/proxy/{host}/", client_handler.HandleProxyEdit)
	mux_client.HandleFunc("/api/saveConfig", client_handler.HandleSaveConfig)
	mux_client.HandleFunc("/", assets.HandleWebAssets)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		user_agent := r.Header.Get("User-Agent")
		is_agent := strings.HasPrefix(user_agent, "go-remote-agent/") || user_agent == ""

		if is_agent {
			mux_agent.ServeHTTP(w, r)
		} else {
			if proxyService, ok := proxy.ProxyServices.Load(r.Host); ok {
				proxyService.(*proxy.Service).HandleRequest(w, r)
			} else {
				mux_client.ServeHTTP(w, r)
			}
		}
	})

	proxy.RegisterFromConfigFile()

	log.Println("Listening on", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalln("failed to ListenAndServe:", err)
		panic(err)
	}
}
