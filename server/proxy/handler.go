package proxy

import (
	"context"
	"errors"
	"log"
	"net/http"
	"remote-agent/biz"
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

func RegisterFromConfigFile() {
	for _, service := range biz.Config.ProxyServices {
		s := Service{
			Host:        service.Host,
			AgentName:   service.AgentName,
			Target:      service.Target,
			ReplaceHost: service.ReplaceHost,
		}
		if err := RegisterService(s); err != nil {
			log.Println("failed to register service:", s, err)
			panic(err)
		}
	}
}

func RegisterService(s Service) error {
	_, existed := ProxyServices.LoadOrStore(s.Host, &s)
	if existed {
		return errors.New("proxy service host already existed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel

	log.Printf("register proxy service: %s --[%s]--> %s", s.Host, s.AgentName, s.Target)
	return nil
}

func KillService(host string) error {
	s, ok := ProxyServices.LoadAndDelete(host)
	if !ok {
		return errors.New("service not found")
	}

	log.Println("kill proxy service:", host)
	s.(*Service).Dispose()
	return nil
}
