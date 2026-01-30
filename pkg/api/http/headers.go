package http

import (
	"net/http"
)

// Headers godoc
// @Summary Headers
// @Description returns a JSON array with the request HTTP headers
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /headers [get]
// @Success 200 {object} http.ArrayResponse
func (s *Server) echoHeadersHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "echoHeadersHandler")
	defer span.End()
	s.JSONResponse(w, r, r.Header)
}
