package http

import (
	"net/http"

	"github.com/stefanprodan/podinfo/pkg/environ"
)

// Headers godoc
// @Summary Headers
// @Description returns a JSON array with the request HTTP headers, with credential values masked, plus X-Spiffe-Id when the client presented a SPIFFE X.509 SVID
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /headers [get]
// @Success 200 {object} http.ArrayResponse
func (s *Server) echoHeadersHandler(w http.ResponseWriter, r *http.Request) {
	s.JSONResponse(w, r, echoedHeaders(r))
}

// echoedHeaders returns the request headers with credentials masked and
// X-Spiffe-Id set only to the verified peer ID, never to a client-supplied value.
func echoedHeaders(r *http.Request) http.Header {
	headers := http.Header(environ.RedactHeaders(r.Header))
	headers.Del(spiffeIDHeader)
	if id, ok := peerSPIFFEID(r); ok {
		headers.Set(spiffeIDHeader, id.String())
	}
	return headers
}

const spiffeIDHeader = "X-Spiffe-Id"
