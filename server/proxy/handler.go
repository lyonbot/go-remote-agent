package proxy

import (
	"net/http"
	"sync"
)

var HostServices = sync.Map{} // map[string]*Service

func HandleProxyRequest(w http.ResponseWriter, r *http.Request) {
	servicePtr, ok := HostServices.Load(r.Host)
	if !ok {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}

	service := servicePtr.(*Service)
	service.HandleRequest(w, r)
}
