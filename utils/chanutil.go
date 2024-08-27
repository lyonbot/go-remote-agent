package utils

import (
	"io"
	"log"
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
			read <- data
		}
	}()

	// write to conn
	go func() {
		defer wg.Done()

		for data := range write {
			if err := c.WriteMessage(websocket.BinaryMessage, data); err != nil {
				log.Println("ws write error:", err)
				// close(write) // "write" shall be closed by outside
				return
			}
		}
	}()

	return &WSConnToChannelsResult{
		Read:  read,
		Write: write,
	}
}
