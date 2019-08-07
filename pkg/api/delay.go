package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"strconv"
	"time"
)

// Delay godoc
// @Summary Delay
// @Description waits for the specified period
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /delay/{seconds} [get]
// @Success 200 {object} api.MapResponse
func (s *Server) delayHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	delay, err := strconv.Atoi(vars["wait"])
	if err != nil {
		s.ErrorResponse(w, r, err.Error(), http.StatusBadRequest)
		return
	}

	time.Sleep(time.Duration(delay) * time.Second)

	s.JSONResponse(w, r, map[string]int{"delay": delay})
}
