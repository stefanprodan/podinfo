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
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "statusHandler")
	defer span.End()

	vars := mux.Vars(r)

	code, err := strconv.Atoi(vars["code"])
	if err != nil {
		s.ErrorResponse(w, r, span, err.Error(), http.StatusBadRequest)
		return
	}

	s.JSONResponseCode(w, r, map[string]int{"status": code}, code)
}
