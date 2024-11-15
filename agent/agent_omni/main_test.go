package agent_omni_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"remote-agent/agent/agent_omni"
	"remote-agent/utils"
	"time"
)

func bytes2hex(data []byte) string {
	return fmt.Sprintf("%x", data)
}

func hex2bytes(data string) []byte {
	out, err := hex.DecodeString(data)
	if err != nil {
		panic(err)
	}
	return out
}

func readWithTimeout(ch <-chan []byte) []byte {
	select {
	case data := <-ch:
		return data
	case <-time.After(time.Second * 5):
		return []byte{}
	}
}

type TestSession struct {
	Session          *agent_omni.PtySession
	TerminateSession context.CancelFunc

	ChFromAgent <-chan []byte
	ChToAgent   chan<- []byte
}

func makeTestSession() (ts *TestSession) {
	ctx, cancel := context.WithCancel(context.Background())

	chToWS := make(chan []byte)
	ws, chFromWS := utils.MakeRWChanTee(chToWS, ctx)
	session := &agent_omni.PtySession{
		Ctx:      ctx,
		Ws:       ws,
		Handlers: make([]func(recv []byte), 256),
	}

	ts = &TestSession{
		Session:          session,
		TerminateSession: cancel,

		ChFromAgent: chFromWS,
		ChToAgent:   chToWS,
	}

	return
}

// usage: go ts.Run()
func (ts *TestSession) Run() {
	ts.Session.SetupProxy()
	ts.Session.Run()
}
