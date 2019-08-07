package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var wsCon = websocket.Upgrader{}

// EchoWS godoc
// @Summary Echo over websockets
// @Description echos content via websockets
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /ws/echo [post]
// @Success 202 {object} api.MapResponse
// Test: go run ./cmd/podcli/* ws localhost:9898/ws/echo
func (s *Server) echoWsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := wsCon.Upgrade(w, r, nil)
	if err != nil {
		if err != nil {
			s.logger.Warn("websocket upgrade error", zap.Error(err))
			return
		}
	}
	defer c.Close()
	done := make(chan struct{})
	defer close(done)
	in := make(chan interface{})
	defer close(in)
	go s.writeWs(c, in)
	go s.sendHostWs(c, in, done)
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			if !strings.Contains(err.Error(), "close") {
				s.logger.Warn("websocket read error", zap.Error(err))
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
		in <- response
	}
}

func (s *Server) sendHostWs(ws *websocket.Conn, in chan interface{}, done chan struct{}) {
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
			in <- status
		case <-done:
			s.logger.Debug("websocket exit")
			return
		}
	}
}

func (s *Server) writeWs(ws *websocket.Conn, in chan interface{}) {
	for {
		select {
		case msg := <-in:
			if err := ws.WriteJSON(msg); err != nil {
				if !strings.Contains(err.Error(), "close") {
					s.logger.Warn("websocket write error", zap.Error(err))
				}
				return
			}
		}
	}
}
