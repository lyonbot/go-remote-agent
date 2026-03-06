package client_handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"remote-agent/biz"
	"remote-agent/server/proxy"
	"strings"
)

func HandleConfigProxies(w http.ResponseWriter, r *http.Request) {
	if block_if_request_api_key_bad(w, r) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"proxyServerHost": biz.Config.ProxyServerHost,
	})
}

func HandleProxyListAll(w http.ResponseWriter, r *http.Request) {
	if block_if_request_api_key_bad(w, r) {
		return
	}

	list := make([]proxy.ServiceInfo, 0)
	proxy.ProxyServices.Range(func(key, value any) bool {
		list = append(list, value.(*proxy.Service).ServiceInfo)
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

	host := r.PathValue("host")
	if !strings.Contains(host, ".") {
		hostTpl := biz.Config.ProxyServerHost
		host = strings.Replace(hostTpl, "*", host, -1)
		if host == hostTpl {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid host, maybe proxy_server_host is not configured"})
			return
		}
	}

	writeError := func(status int, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	}

	switch r.Method {
	case http.MethodPost:
		srv := proxy.ServiceInfo{
			Host:        host,
			AgentName:   r.PostFormValue("agent_name"),
			AgentId:     r.PostFormValue("agent_id"),
			Target:      r.PostFormValue("target"),
			ReplaceHost: r.PostFormValue("replace_host"),
		}

		srv.Target = strings.TrimSpace(srv.Target)
		if srv.Target == "" {
			writeError(http.StatusBadRequest, errors.New("target is required"))
			return
		}
		if !strings.HasPrefix(srv.Target, "http://") && !strings.HasPrefix(srv.Target, "https://") {
			srv.Target = "http://" + srv.Target
		}
		if srv.AgentId == "" && srv.AgentName == "" {
			writeError(http.StatusBadRequest, errors.New("agent_id or agent_name is required"))
			return
		}
		if err := proxy.RegisterService(srv); err != nil {
			writeError(http.StatusConflict, err)
			return
		}
		biz.Config.ProxyServices = append(biz.Config.ProxyServices, biz.SavedProxyConfig{
			Host:        srv.Host,
			AgentName:   srv.AgentName,
			Target:      srv.Target,
			ReplaceHost: srv.ReplaceHost,
		})

	case http.MethodDelete:
		if err := proxy.KillService(host); err != nil {
			writeError(http.StatusNotFound, err)
			return
		}
		filtered := biz.Config.ProxyServices[:0]
		for _, s := range biz.Config.ProxyServices {
			if s.Host != host {
				filtered = append(filtered, s)
			}
		}
		biz.Config.ProxyServices = filtered

	default:
		writeError(http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
