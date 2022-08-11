package api

import (
	"net/http"

	"os"
)

// Env godoc
// @Summary Environment
// @Description returns the environment variables as a JSON array
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /env [get]
// @Success 200 {object} api.ArrayResponse
func (s *Server) envHandler(w http.ResponseWriter, r *http.Request) {
	s.JSONResponse(w, r, os.Environ())
}
