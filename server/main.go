package server

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"remote-agent/biz"
	"remote-agent/server/assets"
	"strings"

	"github.com/gorilla/websocket"
)

var ws = websocket.Upgrader{
	EnableCompression: true,
	CheckOrigin: func(r *http.Request) bool {
		// accept all origin -- be good with reverse proxies
		return true
	},
}

func RunServer() {
	addr := fmt.Sprintf("%s:%d", biz.Config.Addr, biz.Config.Port)

	http.HandleFunc("/api/agent/{agent_name}", handleAgentTaskStreamRequest)
	http.HandleFunc("/api/agent/{agent_name}/{token}", handleAgentTaskWSRequest)

	http.HandleFunc("/api/client/", handleClientListAll)
	http.HandleFunc("/api/client/{agent_name}/", handleClientListAgent)
	http.HandleFunc("/api/client/{agent_name}/exec/", handleClientExec)
	http.HandleFunc("/api/client/{agent_name}/pty/", handleClientPty)

	http.HandleFunc("/", handleWebAssets)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalln("failed to ListenAndServe:", err)
		panic(err)
	}

	log.Println("Listening on", addr)
}

func handleWebAssets(w http.ResponseWriter, r *http.Request) {
	fsPrefix := "frontend"
	path := r.URL.Path

	if strings.HasSuffix(path, "/") {
		// access homepage of a directory
		path += "index.html"
	} else if _, err := assets.StaticFs.ReadDir(fsPrefix + path); err == nil {
		// is a directory
		w.Header().Set("Location", path+"/")
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	if p, err := assets.StaticFs.ReadFile(fsPrefix + path); err == nil && len(p) > 0 {
		if block_if_request_api_key_bad(w, r) {
			return
		}

		// is a file
		// log.Println("Requesting static file", path)
		ext := filepath.Ext(path)
		w.Header().Add("Content-Type", mime.TypeByExtension(ext))
		w.WriteHeader(http.StatusOK)
		w.Write(p)
		return
	}

	// 404
	http.Error(w, "404 Not Found", http.StatusNotFound)
}
