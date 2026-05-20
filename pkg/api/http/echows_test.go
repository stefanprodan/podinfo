package http

import (
	"net/http/httptest"
	"strings"
	"testing"

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
