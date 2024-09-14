package agent

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"remote-agent/agent/agent_common"
	"remote-agent/agent/agent_pty"
	"remote-agent/agent/agent_upgrade"
	"remote-agent/biz"
	"time"

	"github.com/avast/retry-go"
)

var task_stream = make(chan *biz.AgentNotify, 5)

var ctx, cancel_agent_task_stream = context.WithCancel(context.Background())

func listen() {
	// it is tend to not closing the channel,
	// so after upgrading, this process won't exit and the systemctl will not restart it.
	//
	// defer close(task_stream)

	url := agent_common.GetAgentAPIUrl("")
	run := func() error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", biz.UserAgent)
		req = req.WithContext(ctx)

		client := agent_common.MakeHttpClient()
		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return errors.New("status code: " + resp.Status)
		}

		len_buf := make([]byte, 4)
		for {
			if _, err := io.ReadFull(resp.Body, len_buf); err != nil {
				return err
			}

			len := int(binary.LittleEndian.Uint32(len_buf))
			data := make([]byte, len+2) // +2 for CRLF

			if _, err := io.ReadFull(resp.Body, data); err != nil {
				return err
			}

			msg := &biz.AgentNotify{}
			if _, err := msg.UnmarshalMsg(data[:len]); err != nil {
				return err
			}
			task_stream <- msg
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			retry.Do(run, retry.MaxDelay(time.Second*60))
		}
	}
}

func RunAgent() {
	go listen()
	for msg := range task_stream {
		switch msg.Type {
		case "shell":
			go run_shell(msg)
		case "pty":
			go agent_pty.Run(msg)
		case "upgrade":
			go agent_upgrade.Run(msg, cancel_agent_task_stream)
		}
	}
}
