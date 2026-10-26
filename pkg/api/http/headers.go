package http

import (
	"net/http"
)

// Headers godoc
// @Summary Headers
// @Description returns a JSON array with the request HTTP headers, plus X-Spiffe-Id when the client presented a SPIFFE X.509 SVID
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /headers [get]
// @Success 200 {object} http.ArrayResponse
func (s *Server) echoHeadersHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "echoHeadersHandler")
	defer span.End()

	headers := r.Header.Clone()
	// never echo a client-supplied X-Spiffe-Id, only the verified peer ID
	headers.Del(spiffeIDHeader)
	if id, ok := peerSPIFFEID(r); ok {
		headers.Set(spiffeIDHeader, id.String())
	}
	s.JSONResponse(w, r, headers)
}

const spiffeIDHeader = "X-Spiffe-Id"
