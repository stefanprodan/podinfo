package http

import (
	"net/http"

	"github.com/stefanprodan/podinfo/pkg/environ"
)

// Env godoc
// @Summary Environment
// @Description returns the environment variables as a JSON array, with secret values masked
// @Tags HTTP API
// @Produce json
// @Router /env [get]
// @Router /env [post]
// @Success 200 {object} http.ArrayResponse
func (s *Server) envHandler(w http.ResponseWriter, r *http.Request) {
	s.JSONResponse(w, r, environ.Redacted())
}
