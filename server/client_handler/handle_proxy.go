package client_handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"remote-agent/biz"
	"remote-agent/server/proxy"
	"strings"
)

func HandleProxyListAll(w http.ResponseWriter, r *http.Request) {
	if block_if_request_api_key_bad(w, r) {
		return
	}

	list := make([]proxy.Service, 0)
	proxy.ProxyServices.Range(func(key, value interface{}) bool {
		list = append(list, *value.(*proxy.Service))
		return true
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(list)
}

func HandleProxyEdit(w http.ResponseWriter, r *http.Request) {
	if block_if_request_api_key_bad(w, r) {
		return
	}

	var err error

	host := r.PathValue("host")
	if !strings.Contains(host, ".") {
		hostTpl := biz.Config.ProxyServerHost
		host = strings.Replace(hostTpl, "*", host, -1)
		if host == hostTpl {
			err = errors.New("invalid host or bad proxy_server_host config")
		}
	}

	switch r.Method {
	case http.MethodPost:
		srv := proxy.Service{}
		if err == nil {
			err = json.NewDecoder(r.Body).Decode(&srv)
		}
		if err == nil {
			srv.Target = strings.TrimSpace(srv.Target)
			if srv.Target == "" {
				err = errors.New("target is required")
			} else if !strings.HasPrefix(srv.Target, "http://") && !strings.HasPrefix(srv.Target, "https://") {
				srv.Target = "http://" + srv.Target
			}
		}
		if err == nil {
			srv.Host = host
			err = proxy.RegisterService(srv)
		}
	case http.MethodDelete:
		err = proxy.KillService(host)
	default:
		err = errors.New("method not allowed")
	}

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
