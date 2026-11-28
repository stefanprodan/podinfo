package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

var wsCon = websocket.Upgrader{}

const (
	// wsMaxMessageSize caps a single inbound websocket message so one large
	// frame cannot exhaust process memory.
	wsMaxMessageSize = 1 << 20 // 1 MiB
	// wsReadTimeout is how long the server waits for the next client message or
	// pong before closing an idle connection.
	wsReadTimeout = 60 * time.Second
	// wsWriteTimeout bounds a single write so a slow reader cannot block forever.
	wsWriteTimeout = 10 * time.Second
	// wsPingInterval is how often the server pings the client to keep the
	// connection alive and detect dead peers; it must be shorter than wsReadTimeout.
	wsPingInterval = 30 * time.Second
)

// EchoWS godoc
// @Summary Echo over websockets
// @Description upgrades the connection to a websocket, answers each message with a JSON message holding the time and the content, and sends the hostname every five seconds
// @Tags HTTP API
// @Produce plain
// @Router /ws/echo [get]
// @Success 101 "Switching Protocols"
// @Failure 400 {string} string "not a websocket handshake"
// @Failure 403 {string} string "request origin not allowed"
// @Failure 426 {string} string "websocket not found in the Upgrade header"
// Test: go run ./cmd/podcli ws localhost:9898/ws/echo
func (s *Server) echoWsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := wsCon.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("websocket upgrade error", zap.Error(err), telemetry.Context(r.Context()))
		return
	}

	// Each pong extends the read deadline, so only idle or dead connections time out.
	c.SetReadLimit(wsMaxMessageSize)
	_ = c.SetReadDeadline(time.Now().Add(wsReadTimeout))
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(wsReadTimeout))
	})

	defer c.Close()
	done := make(chan struct{})
	defer close(done)
	in := make(chan interface{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer c.Close()
		s.writeWs(r.Context(), c, in, done)
	}()
	send := func(msg interface{}) bool {
		select {
		case in <- msg:
			return true
		case <-writerDone:
			return false
		case <-done:
			return false
		}
	}
	go s.sendHostWs(r.Context(), send, done)
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			if !strings.Contains(err.Error(), "close") {
				s.logger.Warn("websocket read error", zap.Error(err), telemetry.Context(r.Context()))
			}
			break
		}
		var response = struct {
			Time    time.Time `json:"ts"`
			Message string    `json:"msg"`
		}{
			Time:    time.Now(),
			Message: string(message),
		}
		if !send(response) {
			break
		}
	}
}

func (s *Server) sendHostWs(ctx context.Context, send func(interface{}) bool, done chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			var status = struct {
				Time time.Time `json:"ts"`
				Host string    `json:"server"`
			}{
				Time: time.Now(),
				Host: s.config.Hostname,
			}
			if !send(status) {
				return
			}
		case <-done:
			s.logger.Debug("websocket exit", telemetry.Context(ctx))
			return
		}
	}
}

func (s *Server) writeWs(ctx context.Context, ws *websocket.Conn, in chan interface{}, done chan struct{}) {
	ping := time.NewTicker(wsPingInterval)
	defer ping.Stop()
	for {
		select {
		case msg := <-in:
			_ = ws.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := ws.WriteJSON(msg); err != nil {
				if !strings.Contains(err.Error(), "close") {
					s.logger.Warn("websocket write error", zap.Error(err), telemetry.Context(ctx))
				}
				return
			}
		case <-ping.C:
			_ = ws.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
