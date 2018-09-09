package api

import (
	"net/http"

	"os"
)

func (s *Server) envHandler(w http.ResponseWriter, r *http.Request) {
	s.JSONResponse(w, r, os.Environ())
}
