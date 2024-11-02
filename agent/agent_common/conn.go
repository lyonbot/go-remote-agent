package agent_common

import (
	"crypto/tls"
	"net/http"
	"remote-agent/biz"
	"strings"

	"github.com/gorilla/websocket"
)

func GetAgentAPIUrl(path string) string {
	return biz.Config.BaseUrl + "/api/for_agent/" + biz.Config.Name + path
}

func MakeHttpClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: biz.Config.Insecure,
		},
	}
	client := &http.Client{
		Transport: tr,
	}
	return client
}

func MakeWsConn(taskId string) (*websocket.Conn, error) {
	url := GetAgentAPIUrl("/" + taskId)
	url = strings.Replace(url, "http", "ws", 1)

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: biz.Config.Insecure,
		},
	}
	c, _, err := dialer.Dial(url, nil)

	if err != nil {
		return nil, err
	}

	return c, nil
}
