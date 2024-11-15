package agent_omni

import (
	"context"
	"remote-agent/agent/agent_common"
	"remote-agent/biz"
	"remote-agent/utils"
)

type PtySession struct {
	Ctx context.Context
	Ws  *utils.RWChan

	Handlers []func(recv []byte) // length of 255
}

func (s *PtySession) WriteDebugMessage(data string) {
	s.Ws.Write(utils.PrependBytes([]byte{0xff}, []byte(data)))
}

func (s *PtySession) Write(data []byte) {
	s.Ws.Write(data)
}

func Run(task *biz.AgentNotify) {
	c, err := agent_common.MakeWsConn(task.Id)
	if err != nil {
		return
	}
	ws := utils.MakeRWChanFromWebSocket(c)
	defer ws.Close()

	ctx, cancel := context.WithCancel(ws.Ctx)

	session := &PtySession{
		Ctx:      ctx,
		Ws:       ws,
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
