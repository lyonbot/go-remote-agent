package proxy

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

var ProxyServices = sync.Map{} // map[string]*Service

func HandleProxyRequest(w http.ResponseWriter, r *http.Request) {
	servicePtr, ok := ProxyServices.Load(r.Host)
	if !ok {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}

	service := servicePtr.(*Service)
	service.HandleRequest(w, r)
}

func RegisterService(s Service) error {
	_, existed := ProxyServices.LoadOrStore(s.Host, &s)
	if existed {
		return errors.New("service host already existed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel

	return nil
}

func KillService(host string) error {
	s, ok := ProxyServices.LoadAndDelete(host)
	if !ok {
		return errors.New("service not found")
	}
	s.(*Service).Dispose()
	return nil
}
