package agent_pty_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"remote-agent/agent/agent_pty"
	"remote-agent/utils"
	"sync"
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

func makeTestSession() (session *agent_pty.PtySession, wg *sync.WaitGroup, wsRead chan<- []byte, wsWrite <-chan []byte, cancel context.CancelFunc) {
	var ctx context.Context

	ctx, cancel = context.WithCancel(context.Background())
	a := make(chan []byte, 5)
	b := make(chan []byte, 5)
	wsRead = a
	wsWrite = b
	ws := &utils.WSConnToChannelsResult{
		Read:  a,
		Write: b,
	}
	wg = &sync.WaitGroup{}

	session = &agent_pty.PtySession{
		Ctx:      ctx,
		Ws:       ws,
		Wg:       wg,
		Handlers: make([]func(recv []byte), 255),
	}

	return
}
