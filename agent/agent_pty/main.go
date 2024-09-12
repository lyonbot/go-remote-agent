package agent_pty

import (
	"context"
	"remote-agent/biz"
	"remote-agent/utils"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type PtySession struct {
	Ctx context.Context
	Ws  *utils.WSConnToChannelsResult
	Wg  *sync.WaitGroup

	Handlers []func(recv []byte) // length of 255
}

func (s *PtySession) WriteDebugMessage(data string) {
	s.Ws.Write <- utils.PrependBytes([]byte{0x03}, []byte(data))
}

func (s *PtySession) Write(data []byte) {
	utils.TryWrite(s.Ws.Write, data)
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

	session := &PtySession{
		Ctx:      ctx,
		Ws:       ws,
		Wg:       &wg,
		Handlers: make([]func(recv []byte), 255),
	}

	session.SetupPty()
	session.SetupFileTransfer()
	session.SetupTcpProxy()

	session.Run()
	cancel()
}

func (s *PtySession) Run() {
	for recv := range s.Ws.Read {
		handler := s.Handlers[recv[0]]
		if handler != nil {
			go handler(recv)
		}
	}
}
