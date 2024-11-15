package agent_omni_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"remote-agent/biz"
	"remote-agent/utils"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// GET request is sent via 0x23 (http request) package, and it has no body
func TestProxyHttp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "text/plain")
		w.Header().Add("X-Test", "test")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "method: %s\n", r.Method)
		fmt.Fprintf(w, "body: %s\n", string(body))
	}))
	defer srv.Close()

	ts := makeTestSession()
	defer ts.TerminateSession()
	go ts.Run()

	idBytes := []byte{0xde, 0xad, 0xbe, 0xef}

	conn_req := biz.ProxyHttpRequest{
		Method: "GET",
		URL:    srv.URL,
		Headers: []biz.ProxyHttpHeader{
			{Name: "Content-Type", Value: "text/plain"},
		},
	}
	conn_req_bytes, _ := conn_req.MarshalMsg(nil)
	connect_command := utils.JoinBytes2(0x23, idBytes, conn_req_bytes)

	// -------------------------------------
	// 1. send http request
	ts.ChToAgent <- connect_command
	if recv := readWithTimeout(ts.ChFromAgent); bytes2hex(recv[:5]) != "23deadbeef" {
		t.Fatalf("did not recv http dial result: %s", bytes2hex(recv))
	} else {
		dial_result := biz.ProxyHttpResponse{}
		if _, err := dial_result.UnmarshalMsg(recv[5:]); err != nil {
			t.Fatalf("failed to unmarshal http dial result: %s", err.Error())
		}
		Assert(t, dial_result.StatusCode == 200, "http status code 200")
		Assert(t, dial_result.ConnectionError == "", "no connection error")

		found_x_test := false
		for _, h := range dial_result.Headers {
			if h.Name == "X-Test" && h.Value == "test" {
				found_x_test = true
				break
			}
		}

		Assert(t, found_x_test, "http header X-Test")
	}

	// -------------------------------------
	// 2. recv http response data
	recv_done := make(chan struct{})
	recv_buf := make([]byte, 0)
	go func() {
		defer close(recv_done)
		for {
			chunk := readWithTimeout(ts.ChFromAgent)
			if bytes2hex(chunk[:5]) == "22deadbeef" {
				break
			}
			if bytes2hex(chunk[:5]) == "21deadbeef" {
				recv_buf = append(recv_buf, chunk[5:]...)
				continue
			}
			t.Errorf("unknown package %s", bytes2hex(chunk))
		}
	}()

	select {
	case <-recv_done:
	case <-time.After(time.Second * 5):
		t.Fatalf("recv data timeout. no 0x22 package")
	}

	expected_data := "method: GET\nbody: \n"
	if string(recv_buf) != expected_data {
		t.Fatalf("http response not matched:\n%s", string(recv_buf))
	}
}

// POST data is sent via 0x23 (http request) package, not 0x21
func TestProxyHttpPOST(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "text/plain")
		w.Header().Add("X-Test", "test")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "method: %s\n", r.Method)
		fmt.Fprintf(w, "body: %s\n", string(body))
	}))
	defer srv.Close()

	ts := makeTestSession()
	defer ts.TerminateSession()
	go ts.Run()

	idBytes := []byte{0xde, 0xad, 0xbe, 0xef}

	conn_req := biz.ProxyHttpRequest{
		Method: "POST",
		URL:    srv.URL,
		Headers: []biz.ProxyHttpHeader{
			{Name: "Content-Type", Value: "text/plain"},
		},
		Body: []byte("hello world"),
	}
	conn_req_bytes, _ := conn_req.MarshalMsg(nil)
	connect_command := utils.JoinBytes2(0x23, idBytes, conn_req_bytes)

	// -------------------------------------
	// 1. send http request
	ts.ChToAgent <- connect_command
	if recv := readWithTimeout(ts.ChFromAgent); bytes2hex(recv[:5]) != "23deadbeef" {
		t.Fatalf("did not recv http dial result: %s", bytes2hex(recv))
	} else {
		dial_result := biz.ProxyHttpResponse{}
		if _, err := dial_result.UnmarshalMsg(recv[5:]); err != nil {
			t.Fatalf("failed to unmarshal http dial result: %s", err.Error())
		}
		if dial_result.StatusCode != 200 {
			t.Fatalf("http status code not 200: %d", dial_result.StatusCode)
		}
		if dial_result.ConnectionError != "" {
			t.Fatalf("http connection error: %s", dial_result.ConnectionError)
		}

		found_x_test := false
		for _, h := range dial_result.Headers {
			if h.Name == "X-Test" && h.Value == "test" {
				found_x_test = true
				break
			}
		}
		if !found_x_test {
			t.Fatalf("http header not found: X-Test")
		}
	}

	// -------------------------------------
	// 2. recv http response data
	recv_done := make(chan struct{})
	recv_buf := make([]byte, 0)
	go func() {
		defer close(recv_done)
		for {
			chunk := readWithTimeout(ts.ChFromAgent)
			if bytes2hex(chunk[:5]) == "22deadbeef" {
				break
			}
			if bytes2hex(chunk[:5]) == "21deadbeef" {
				recv_buf = append(recv_buf, chunk[5:]...)
				continue
			}
			t.Errorf("unknown package %s", bytes2hex(chunk))
		}
	}()

	select {
	case <-recv_done:
	case <-time.After(time.Second * 5):
		t.Fatalf("recv data timeout. no 0x22 package")
	}

	expected_data := "method: POST\nbody: hello world\n"
	if string(recv_buf) != expected_data {
		t.Fatalf("http response not matched:\n%s", string(recv_buf))
	}
}

// server may send 0x22 package to stop a http response.
// it works for SSE very well
func TestProxyHttpAbort(t *testing.T) {
	chunkSentCount := 0
	chunkSendingEnded := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		w.WriteHeader(http.StatusOK)

		for {
			select {
			case <-r.Context().Done():
				chunkSendingEnded <- struct{}{}
				return
			case <-time.After(10 * time.Millisecond):
				chunkSentCount++
				w.(io.Writer).Write([]byte("h"))
				w.(http.Flusher).Flush()
			}
		}
	}))
	defer srv.Close()

	ts := makeTestSession()
	defer ts.TerminateSession()
	go ts.Run()

	idBytes := []byte{0xde, 0xad, 0xbe, 0xef}

	conn_req := biz.ProxyHttpRequest{
		Method: "GET",
		URL:    srv.URL,
	}
	conn_req_bytes, _ := conn_req.MarshalMsg(nil)
	connect_command := utils.JoinBytes2(0x23, idBytes, conn_req_bytes)

	// -------------------------------------
	// 1. send http request
	ts.ChToAgent <- connect_command
	if recv := readWithTimeout(ts.ChFromAgent); bytes2hex(recv[:5]) != "23deadbeef" {
		t.Fatalf("did not recv http dial result: %s", bytes2hex(recv))
	} else {
		dial_result := biz.ProxyHttpResponse{}
		if _, err := dial_result.UnmarshalMsg(recv[5:]); err != nil {
			t.Fatalf("failed to unmarshal http dial result: %s", err.Error())
		}
		if dial_result.StatusCode != 200 {
			t.Fatalf("http status code not 200: %d", dial_result.StatusCode)
		}
		if dial_result.ConnectionError != "" {
			t.Fatalf("http connection error: %s", dial_result.ConnectionError)
		}

		content_type_matches := false
		for _, h := range dial_result.Headers {
			if h.Name == "Content-Type" && h.Value == "text/event-stream" {
				content_type_matches = true
				break
			}
		}
		if !content_type_matches {
			t.Fatalf("http header not match: Content-Type")
		}
	}

	// -------------------------------------
	// 2. recv http response data
	recv_buf := make([]byte, 0)
	recv_end := make(chan struct{})
	go func() {
		for {
			chunk := readWithTimeout(ts.ChFromAgent)
			if bytes2hex(chunk[:5]) == "22deadbeef" {
				recv_end <- struct{}{}
				return
			}
			if bytes2hex(chunk[:5]) == "21deadbeef" {
				recv_buf = append(recv_buf, chunk[5:]...)
				continue
			}
			t.Errorf("unknown package %s", bytes2hex(chunk))
			recv_end <- struct{}{}
		}
	}()

	time.Sleep(150 * time.Millisecond)
	ts.ChToAgent <- hex2bytes("22deadbeef") // close
	select {
	case <-recv_end:
	case <-time.After(time.Second * 5):
		t.Fatalf("proxy not sending 0x22 package (mock server not stop sending data)")
	}

	select {
	case <-chunkSendingEnded:
	case <-time.After(time.Second * 5):
		t.Fatalf("mock server chunk sending not ended")
	}

	if len(recv_buf) < chunkSentCount-3 || len(recv_buf) > chunkSentCount+3 {
		t.Fatalf("http response length not matched:\nexpect %d, got %s(%d)", chunkSentCount, string(recv_buf), len(recv_buf))
	}
}

func TestProxyHttpWebSocket(t *testing.T) {
	chunkSentCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test-Client") != "test" {
			t.Fatalf("unexpected X-Test-Client header: %s", r.Header.Get("X-Test-Client"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ws := websocket.Upgrader{}
		conn, err := ws.Upgrade(w, r, http.Header{
			"X-Test-Server": []string{"test"},
		})
		if err != nil {
			t.Fatalf("failed to upgrade to websocket: %v", err)
		}

		defer conn.Close()
		conn.WriteMessage(websocket.BinaryMessage, []byte("binary"))
		conn.WriteMessage(websocket.TextMessage, []byte("text"))

		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			chunkSentCount++
			conn.WriteMessage(messageType, append([]byte("recv: "), data...))
		}
	}))
	defer srv.Close()

	ts := makeTestSession()
	defer ts.TerminateSession()
	go ts.Run()

	idBytes := []byte{0xde, 0xad, 0xbe, 0xef}

	conn_req := biz.ProxyHttpRequest{
		Method: "GET",
		URL:    strings.Replace(srv.URL, "http://", "ws://", 1),
		Headers: []biz.ProxyHttpHeader{
			{Name: "X-Test-Client", Value: "test"},
		},
	}
	conn_req_bytes, _ := conn_req.MarshalMsg(nil)
	connect_command := utils.JoinBytes2(0x23, idBytes, conn_req_bytes)

	// -------------------------------------
	// 1. send http request
	ts.ChToAgent <- connect_command
	if recv := readWithTimeout(ts.ChFromAgent); bytes2hex(recv[:5]) != "23deadbeef" {
		t.Fatalf("did not recv http dial result: %s", bytes2hex(recv))
	} else {
		dial_result := biz.ProxyHttpResponse{}
		if _, err := dial_result.UnmarshalMsg(recv[5:]); err != nil {
			t.Fatalf("failed to unmarshal http dial result: %s", err.Error())
		}

		Assert(t, dial_result.ConnectionError == "", "no connection error")

		found_x_test := false
		for _, h := range dial_result.Headers {
			if h.Name == "X-Test-Server" && h.Value == "test" {
				found_x_test = true
				break
			}
		}

		Assert(t, found_x_test, "http header X-Test")
	}

	// -------------------------------------
	// 2. handle data
	wsRecvMessages := make(chan []byte, 10)
	go func() {
		for data := range ts.ChFromAgent {
			if data[0] == 0x21 {
				wsRecvMessages <- data[5:]
			}

			if data[0] == 0x22 {
				close(wsRecvMessages)
				break
			}
		}
	}()

	msg1 := <-wsRecvMessages
	Assert(t, string(msg1) == "\x02binary", "ws message 1")

	msg2 := <-wsRecvMessages
	Assert(t, string(msg2) == "\x01text", "ws message 2")

	// send a message and echo back
	ts.ChToAgent <- utils.JoinBytes2(0x21, idBytes, []byte("\x01hello"))
	msg3 := <-wsRecvMessages
	Assert(t, string(msg3) == "\x01recv: hello", "ws message 3")

	// -------------------------------------
	// 3. close
	ts.ChToAgent <- utils.JoinBytes2(0x22, idBytes)
	select {
	case <-wsRecvMessages:
	case <-time.After(time.Second * 5):
		t.Fatalf("did not recv close message")
	}
}

func Assert(t *testing.T, cond bool, msg string) {
	if !cond {
		t.Fatalf("assert failed: %s", msg)
	}
}
