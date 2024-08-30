package client_handler

import (
	"net/http"
	"remote-agent/biz"
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

func is_request_api_key_good(r *http.Request) bool {
	correct_key := biz.Config.APIKey
	if correct_key == "" {
		return true
	}

	api_key_2 := r.FormValue("api_key")
	if api_key_2 != "" {
		return api_key_2 == correct_key
	}

	api_key := r.Header.Get("X-API-Key")
	if api_key != "" {
		return api_key == correct_key
	}

	auth := r.Header.Get("Authorization")
	if auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			auth = auth[7:]
		}
		return auth == correct_key
	}

	return false
}
func block_if_request_api_key_bad(w http.ResponseWriter, r *http.Request) (blocked bool) {
	if !is_request_api_key_good(r) {
		w.Header().Add("WWW-Authenticate", `Bearer realm="go-remote-agent"`)
		http.Error(w, "API key is invalid", http.StatusUnauthorized)
		return true
	}
	return false
}
