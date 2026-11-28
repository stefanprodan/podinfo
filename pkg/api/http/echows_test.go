// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestEchoWsHandler(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/ws/echo", srv.echoWsHandler)
	server := httptest.NewServer(srv.router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/echo"
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer resp.Body.Close()
	defer ws.Close()

	msg := "hello websocket"
	if err := ws.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	_, p, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if len(p) == 0 {
		t.Error("received empty message")
	}
}

func TestEchoWsReadLimit(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/ws/echo", srv.echoWsHandler)
	server := httptest.NewServer(srv.router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/echo"
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer resp.Body.Close()
	defer ws.Close()

	oversize := make([]byte, wsMaxMessageSize+1)
	if err := ws.WriteMessage(websocket.TextMessage, oversize); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// The server answers with a 1009 (message too big) close frame. The loop
	// skips the periodic status frames, and the read deadline bounds it.
	if err := ws.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		if _, _, err = ws.ReadMessage(); err != nil {
			break
		}
	}
	if !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
		t.Errorf("expected close code %d (message too big), got: %v", websocket.CloseMessageTooBig, err)
	}
}

type failingConn struct {
	net.Conn
	fail *atomic.Bool
}

func (c failingConn) Write(p []byte) (int, error) {
	if c.fail.Load() {
		return 0, errors.New("write failed")
	}
	return c.Conn.Write(p)
}

type failingHijacker struct {
	http.ResponseWriter
	fail *atomic.Bool
}

func (w failingHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, brw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err != nil {
		return nil, nil, err
	}
	return failingConn{Conn: conn, fail: w.fail}, brw, nil
}

// goroutineID returns the ID of the calling goroutine.
func goroutineID() string {
	buf := make([]byte, 64)
	return strings.Fields(string(buf[:runtime.Stack(buf, false)]))[1]
}

// startedBy counts the goroutines running fn that the goroutine with the given ID started.
func startedBy(id, fn string) int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	var count int
	for g := range strings.SplitSeq(string(buf), "\n\n") {
		if strings.Contains(g, fn) && strings.Contains(g, " in goroutine "+id+"\n") {
			count++
		}
	}
	return count
}

func TestEchoWsWriterFailureReleasesHandler(t *testing.T) {
	srv := NewMockServer()
	var fail atomic.Bool
	handler := make(chan string, 1)
	returned := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(returned)
		handler <- goroutineID()
		srv.echoWsHandler(failingHijacker{ResponseWriter: w, fail: &fail}, r)
	}))
	defer server.Close()

	ws, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer resp.Body.Close()
	defer ws.Close()

	if err := ws.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, _, err := ws.ReadMessage(); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	id := <-handler
	started := func() int {
		return startedBy(id, "(*Server).writeWs(") + startedBy(id, "(*Server).sendHostWs(")
	}
	waitFor(t, "the websocket goroutines to start", func() bool { return started() == 2 })

	fail.Store(true)
	waitFor(t, "the handler to return after the writer failed", func() bool {
		select {
		case <-returned:
			return true
		default:
			_ = ws.WriteMessage(websocket.TextMessage, []byte("ping"))
			return false
		}
	})
	waitFor(t, "the websocket goroutines to stop", func() bool { return started() == 0 })
}
