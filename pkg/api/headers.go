package api

import (
	"net/http"
)

func (s *Server) echoHeadersHandler(w http.ResponseWriter, r *http.Request) {
	s.JSONResponse(w, r, r.Header)
}
