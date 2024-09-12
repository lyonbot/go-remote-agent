package agent_pty

import (
	"context"
	"remote-agent/biz"
	"remote-agent/utils"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type ptySessionRecv struct {
	ctx context.Context
	ws  *utils.WSConnToChannelsResult
	wg  *sync.WaitGroup

	handlers []func(recv []byte) // length of 255
}

func (s *ptySessionRecv) WriteDebugMessage(data string) {
	s.ws.Write <- utils.PrependBytes([]byte{0x03}, []byte(data))
}

func Run(task *biz.AgentNotify) {
	url := biz.Config.BaseUrl + "/api/agent/" + biz.Config.Name + "/" + task.Id
	url = strings.Replace(url, "http", "ws", 1)
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return
	}
	defer c.Close()

	wg := sync.WaitGroup{}
	ws := utils.WSConnToChannels(c, &wg)
	defer func() {
		utils.TryClose(ws.Write)
		ws.Write = nil // for unclosed pty
		wg.Wait()
	}()

	ctx, cancel := context.WithCancel(context.Background())

	session := &ptySessionRecv{
		ctx:      ctx,
		ws:       ws,
		wg:       &wg,
		handlers: make([]func(recv []byte), 255),
	}

	session.setupPty()
	session.setupFileTransfer()

	for recv := range ws.Read {
		handler := session.handlers[recv[0]]
		if handler != nil {
			handler(recv)
		}
	}
	cancel()
}
