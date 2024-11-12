package agent_omni

import (
	"context"
	"remote-agent/agent/agent_common"
	"remote-agent/biz"
	"remote-agent/utils"
	"sync"
)

type PtySession struct {
	Ctx context.Context
	Ws  *utils.WSConnToChannelsResult
	Wg  *sync.WaitGroup

	Handlers []func(recv []byte) // length of 255
}

func (s *PtySession) WriteDebugMessage(data string) {
	s.Ws.Write <- utils.PrependBytes([]byte{0xff}, []byte(data))
}

func (s *PtySession) Write(data []byte) {
	utils.TryWrite(s.Ws.Write, data)
}

func Run(task *biz.AgentNotify) {
	c, err := agent_common.MakeWsConn(task.Id)
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
		Handlers: make([]func(recv []byte), 256),
	}

	session.SetupPty()
	session.SetupFileTransfer()
	session.SetupProxy()

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
