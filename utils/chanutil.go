package utils

import (
	"io"
	"sync"

	"github.com/gorilla/websocket"
)

// ReadToChannel reads from r to ch
// and close ch when r is closed
func ReaderToChannel(ch chan<- []byte, r io.Reader) {
	defer close(ch)
	for {
		buf := make([]byte, 1024)
		n, err := (r).Read(buf)
		if err != nil {
			break
		}
		ch <- buf[:n]
	}
}

func MakeChanIf[Result any](cond bool, size int) chan Result {
	if cond {
		return make(chan Result, size)
	}

	return nil
}

// only close ch if ch is not nil
func TryClose[Result any](ch chan<- Result) {
	if ch != nil {
		close(ch)
	}
}

// only write data to ch if ch is not nil
func TryWrite[Result any](ch chan<- Result, data Result) {
	if ch != nil {
		ch <- data
	}
}

func ChannelToWriter(ch <-chan []byte, w io.WriteCloser) {
	defer w.Close()
	for data := range ch {
		if _, err := w.Write(data); err != nil {
			return
		}
	}
}

type WSConnToChannelsResult struct {
	Read  <-chan []byte
	Write chan<- []byte
}

// convert websocket conn to channels
//
// - Read is read binary data from conn
// - Write is write binary data to conn
func WSConnToChannels(c *websocket.Conn, wg *sync.WaitGroup) *WSConnToChannelsResult {
	read := make(chan []byte, 5)
	write := make(chan []byte, 5)
	closed := make(chan struct{}, 1)

	wg.Add(2)

	// read from conn
	go func() {
		defer wg.Done()
		defer close(read)

		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				break
			}

			if len(data) > 0 {
				read <- data
			}
		}

		closed <- struct{}{}
		close(closed)
	}()

	// write to conn
	go func() {
		defer wg.Done()
		defer c.Close()

		for {
			select {
			case data, ok := <-write:
				if !ok {
					// write channel closed
					return
				}
				if err := c.WriteMessage(websocket.BinaryMessage, data); err != nil {
					return
				}

			case <-closed:
				return
			}
		}
	}()

	return &WSConnToChannelsResult{
		Read:  read,
		Write: write,
	}
}
