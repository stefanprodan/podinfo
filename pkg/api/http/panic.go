package http

import (
	"net/http"
	"os"
)

// Panic godoc
// @Summary Panic
// @Description crashes the process with exit code 255
// @Tags HTTP API
// @Router /panic [get]
func (s *Server) panicHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Panic command received")
	os.Exit(255)
}
