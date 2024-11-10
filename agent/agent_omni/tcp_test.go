package agent_omni_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"remote-agent/utils"
	"sync"
	"testing"
)

func TestAgentPtyTcp(t *testing.T) {
	echoServer, err := startEchoTcpServer()
	if err != nil {
		t.Fatalf("failed to start echo server: %v", err)
	}
	defer echoServer.Stop()

	session, wg, wsRead, wsWrite, cancel := makeTestSession()
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		session.SetupTcpProxy()
		session.Run()
	}()

	idBytes := []byte{0xde, 0xad, 0xbe, 0xef}
	wsRead <- utils.JoinBytes2(
		0x20,
		idBytes, // id
		binary.LittleEndian.AppendUint16(nil, uint16(echoServer.Port)), // port
		[]byte("127.0.0.1"), // addr
	)

	// shall connect to 127.0.0.1:80
	if recv := readWithTimeout(wsWrite); bytes2hex(recv[:6]) != "20deadbeef00" {
		t.Fatalf("failed to connect: %s", bytes2hex(recv))
	}

	// send data
	wsRead <- hex2bytes("21deadbeefdeadbeef")

	// recv echo data
	if recv := readWithTimeout(wsWrite); bytes2hex(recv) != "21deadbeefdeadbeef" {
		t.Fatalf("failed to recv data 0: %s", bytes2hex(recv))
	}

	// recv
	echoServer.Write(hex2bytes("12345678"))
	if recv := readWithTimeout(wsWrite); bytes2hex(recv) != "21deadbeef12345678" {
		t.Fatalf("failed to recv data 1: %s", bytes2hex(recv))
	}

	// recv closing
	echoServer.Stop()
	if recv := readWithTimeout(wsWrite); bytes2hex(recv) != "22deadbeef" {
		t.Fatalf("failed to recv closing: %s", bytes2hex(recv))
	}
}

type MockServer struct {
	Port  int
	Stop  context.CancelFunc
	Write func(data []byte)
}

// start a mock echo server, it accepts data up to 4k
func startEchoTcpServer() (server *MockServer, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err != nil {
		return
	}

	var l *net.TCPListener
	if l, err = net.ListenTCP("tcp", a); err != nil {
		return
	}

	wg := &sync.WaitGroup{}
	conns := sync.Map{}

	server = &MockServer{
		Port: l.Addr().(*net.TCPAddr).Port,
		Stop: func() {
			l.Close()
			conns.Range(func(key, value interface{}) bool {
				conn := value.(*net.TCPConn)
				conn.Close()
				return true
			})
			wg.Wait()
		},
		Write: func(data []byte) {
			conns.Range(func(key, value interface{}) bool {
				conn := value.(*net.TCPConn)
				conn.Write(bytes.Clone(data))
				return true
			})
		},
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer l.Close()

		for {
			conn, err := l.AcceptTCP()
			if err != nil {
				return
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()

				key := conn.RemoteAddr().String()
				conns.Store(key, conn)
				defer conns.Delete(key)

				for {
					data := make([]byte, 4096)
					n, err := conn.Read(data)
					if err != nil {
						return
					}

					if _, err := conn.Write(data[:n]); err != nil {
						return
					}
				}
			}()
		}
	}()

	return
}
