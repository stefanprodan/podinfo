package http

import (
	"net/http"
	"os"

	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

// Panic godoc
// @Summary Panic
// @Description crashes the process with exit code 255
// @Tags HTTP API
// @Router /panic [get]
func (s *Server) panicHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Panic command received", telemetry.Context(r.Context()))
	os.Exit(255)
}
