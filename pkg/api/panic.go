package api

import (
	"net/http"
)

func (s *Server) panicHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Panic("Panic command received")
}
