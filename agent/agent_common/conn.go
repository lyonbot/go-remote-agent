package agent_common

import (
	"crypto/tls"
	"net/http"
	"remote-agent/biz"
	"strings"

	"github.com/gorilla/websocket"
)

func GetAgentAPIUrl(path string) string {
	return biz.Config.BaseUrl + "/api/agent/" + biz.Config.Name + path
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
	headers := http.Header{}
	headers.Set("User-Agent", biz.UserAgent)

	c, _, err := dialer.Dial(url, headers)

	if err != nil {
		return nil, err
	}

	return c, nil
}
