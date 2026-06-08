package http

import (
	"net/http/httptest"
	"strings"
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
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
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

// TestEchoWsReadLimit verifies the server caps inbound message size: a message
// larger than wsMaxMessageSize must cause the server to close the connection
// instead of buffering it into memory.
func TestEchoWsReadLimit(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/ws/echo", srv.echoWsHandler)
	server := httptest.NewServer(srv.router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/echo"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer ws.Close()

	// A message just over the limit must not be echoed; the server closes it.
	oversize := make([]byte, wsMaxMessageSize+1)
	if err := ws.WriteMessage(websocket.TextMessage, oversize); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Read until the connection errors. With the read limit in place the server
	// sends a 1009 (message too big) close frame; without it the server would
	// instead echo the oversize payload back. The 5s deadline bounds the loop.
	// Status frames from the periodic ticker (err == nil) are skipped.
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		if _, _, err = ws.ReadMessage(); err != nil {
			break
		}
	}
	if !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
		t.Errorf("expected close code %d (message too big), got: %v", websocket.CloseMessageTooBig, err)
	}
}
