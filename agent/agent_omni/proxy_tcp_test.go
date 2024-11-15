package agent_omni_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"remote-agent/utils"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestProxyTcp(t *testing.T) {
	echoServer, err := startEchoTcpServer()
	if err != nil {
		t.Fatalf("failed to start echo server: %v", err)
	}
	defer echoServer.Stop()

	ts := makeTestSession()
	defer ts.TerminateSession()
	go ts.Run()

	idBytes := []byte{0xde, 0xad, 0xbe, 0xef}
	connect_command := utils.JoinBytes2(
		0x20,
		idBytes, // id
		binary.LittleEndian.AppendUint16(nil, uint16(echoServer.Port)), // port
		[]byte("127.0.0.1"), // addr
	)

	// -------------------------------------
	// 1. disconnect by remote

	// shall connect to 127.0.0.1:80
	ts.ChToAgent <- connect_command
	if recv := readWithTimeout(ts.ChFromAgent); bytes2hex(recv[:6]) != "20deadbeef00" {
		t.Fatalf("failed to connect: %s", bytes2hex(recv))
	}

	// send data
	ts.ChToAgent <- hex2bytes("21deadbeefdeadbeef")

	// recv echo data
	if recv := readWithTimeout(ts.ChFromAgent); bytes2hex(recv) != "21deadbeefdeadbeef" {
		t.Fatalf("failed to recv data 0: %s", bytes2hex(recv))
	}

	// recv
	echoServer.Write(hex2bytes("12345678"))
	if recv := readWithTimeout(ts.ChFromAgent); bytes2hex(recv) != "21deadbeef12345678" {
		t.Fatalf("failed to recv data 1: %s", bytes2hex(recv))
	}

	// recv closing
	echoServer.KickAllClients()
	if recv := readWithTimeout(ts.ChFromAgent); bytes2hex(recv) != "22deadbeef" {
		t.Fatalf("failed to recv closing: %s", bytes2hex(recv))
	}

	// -------------------------------------
	// 2. disconnect by user

	ts.ChToAgent <- connect_command
	if recv := readWithTimeout(ts.ChFromAgent); bytes2hex(recv[:6]) != "20deadbeef00" {
		t.Fatalf("failed to connect: %s", bytes2hex(recv))
	}
	ts.ChToAgent <- hex2bytes("21deadbeefdeadbeef")
	if recv := readWithTimeout(ts.ChFromAgent); bytes2hex(recv) != "21deadbeefdeadbeef" {
		t.Fatalf("failed to recv data 0: %s", bytes2hex(recv))
	}

	ts.ChToAgent <- hex2bytes("22deadbeef") // close
	if recv := readWithTimeout(ts.ChFromAgent); bytes2hex(recv) != "22deadbeef" {
		t.Fatalf("failed to recv closing: %s", bytes2hex(recv))
	}
	time.Sleep(100 * time.Millisecond)
	if count := echoServer.OnlineCount.Load(); count != 0 {
		t.Fatalf("server online count not zero: %d", count)
	}
}

type MockServer struct {
	Port           int
	Stop           context.CancelFunc
	Write          func(data []byte)
	OnlineCount    atomic.Int32
	KickAllClients func()
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
			server.KickAllClients()
		},
		Write: func(data []byte) {
			conns.Range(func(key, value interface{}) bool {
				conn := value.(*net.TCPConn)
				conn.Write(bytes.Clone(data))
				return true
			})
		},
		KickAllClients: func() {
			conns.Range(func(key, value interface{}) bool {
				conn := value.(*net.TCPConn)
				conn.Close()
				return true
			})
		},
	}
	// log.Println("echoServer start listening on", server.Port)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer l.Close()

		for {
			conn, err := l.AcceptTCP()
			if err != nil {
				return
			}

			server.OnlineCount.Add(1)
			wg.Add(1)
			go func() {
				defer server.OnlineCount.Add(-1)
				defer wg.Done()
				defer conn.Close()

				key := conn.RemoteAddr().String()
				conns.Store(key, conn)
				defer conns.Delete(key)

				// defer log.Println("echoServer disconnected from", key)
				// log.Println("echoServer accepted tcp connection from", key)

				io.Copy(conn, conn)
			}()
		}
	}()

	return
}
