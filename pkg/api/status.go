package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"strconv"
)

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	code, err := strconv.Atoi(vars["code"])
	if err != nil {
		s.ErrorResponse(w, r, err.Error(), http.StatusBadRequest)
		return
	}

	s.JSONResponseCode(w, r, map[string]int{"status": code}, code)
}
