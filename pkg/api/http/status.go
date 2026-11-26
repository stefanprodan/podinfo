package http

import (
	"net/http"

	"strconv"

	"github.com/gorilla/mux"
)

// Status godoc
// @Summary Status code
// @Description sets the response status code to the specified code
// @Tags HTTP API
// @Accept json
// @Produce json
// @Param code path int true "status code to return"
// @Router /status/{code} [get]
// @Success 200 {object} http.MapResponse
// @Failure 400 {object} http.ErrorMessage "invalid status code"
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	code, err := strconv.Atoi(vars["code"])
	if err != nil || code < 100 || code > 999 {
		s.ErrorResponse(w, r, "invalid status code", http.StatusBadRequest)
		return
	}

	s.JSONResponseCode(w, r, map[string]int{"status": code}, code)
}
