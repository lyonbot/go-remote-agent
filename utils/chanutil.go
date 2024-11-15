package utils

import (
	"context"
	"errors"
	"io"

	"github.com/gorilla/websocket"
)

// ReadToChannel reads from r to ch
// and close ch when r is closed
func ReaderToChannel(ch chan<- []byte, r io.Reader, bufferSize int) {
	defer close(ch)
	for {
		buf := make([]byte, bufferSize)
		n, err := (r).Read(buf)
		if n > 0 {
			ch <- buf[:n]
		}
		if err != nil {
			break
		}
	}
}

// Read channel until context is done or the channel is closed.
// It returns true if reading was stopped by context, false if the channel was closed.
func ReadChanUnderContext[Data any](ctx context.Context, ch <-chan Data, callback func(data Data)) (stopped_by_context bool) {
	ctxDone := ctx.Done()
	for {
		select {
		case data, ok := <-ch:
			if !ok {
				// Channel is closed, hence stopped by the channel being closed
				return false
			}
			callback(data)
		case <-ctxDone:
			// Stopped by context
			return true
		}
	}
}

type RWChan struct {
	Ctx   context.Context
	Close context.CancelFunc

	Read  <-chan []byte
	write chan<- []byte // internal - do not close me!
}

func (r *RWChan) Write(data []byte) error {
	if r.IsClosed() {
		return errors.New("closed")
	}
	r.write <- data
	return nil
}

func (r *RWChan) IsClosed() bool {
	return r.Ctx.Err() != nil
}

// you may act as a "server", sending and receiving data from conn(aka. agent)
//
// connection is closed when chWriteToConn is closed
func MakeRWChanTee(chToConn <-chan []byte, parentCtx context.Context) (conn *RWChan, chFromConn <-chan []byte) {
	_chanToConn := make(chan []byte)
	_chanFromConn := make(chan []byte)

	ctx, cancel := context.WithCancel(parentCtx)
	c := &RWChan{
		Read:  _chanToConn,
		write: _chanFromConn,
		Ctx:   ctx,
		Close: cancel,
	}

	go func() {
		// ---- forward data from mock server to agent
		for data := range chToConn {
			if !c.IsClosed() {
				_chanToConn <- data
			}
		}

		// ---- mock server closed
		cancel()
	}()

	go func() {
		<-ctx.Done()
		close(_chanToConn)
		close(_chanFromConn)
	}()

	return c, _chanFromConn
}

// convert websocket conn to channels
//
// 1. Read is read binary data from conn
// 2. Write is write binary data to conn
// 3. Will close conn
func MakeRWChanFromWebSocket(conn *websocket.Conn) *RWChan {
	read := make(chan []byte)
	write := make(chan []byte)

	ctx, cancel := context.WithCancel(context.Background())

	wsc := &RWChan{
		Read:  read,
		write: write,
		Ctx:   ctx,
		Close: cancel,
	}

	// read from conn
	go func() {
		defer close(read)

		for {
			_, data, err := conn.ReadMessage()
			if len(data) > 0 {
				read <- data
			}
			if err != nil {
				cancel()
				break
			}
			if wsc.IsClosed() {
				break
			}
		}
	}()

	// write to conn
	go func() {
		defer conn.Close()
		defer close(write)

		for {
			select {
			case data := <-write:
				if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
					cancel()
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	return wsc
}
