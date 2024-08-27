package agent

import (
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"remote-agent/biz"
	"time"

	"github.com/avast/retry-go"
)

var task_stream = make(chan *biz.AgentNotify, 5)

func listen() {
	url := biz.Config.BaseUrl + "/api/agent/" + biz.Config.Name
	run := func() error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}

		client := http.Client{}
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
		retry.Do(run, retry.MaxDelay(time.Second*60))
	}
}

func RunAgent() {
	go listen()
	for {
		msg := <-task_stream

		if msg.Type == "shell" {
			go run_shell(msg)
		}
	}
}
