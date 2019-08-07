package api

import (
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// Chunked godoc
// @Summary Chunked transfer encoding
// @Description uses transfer-encoding type chunked to give a partial response and then waits for the specified period
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /chunked/{seconds} [get]
// @Success 200 {object} api.MapResponse
func (s *Server) chunkedHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	delay, err := strconv.Atoi(vars["wait"])
	if err != nil {
		delay = rand.Intn(int(s.config.HttpServerTimeout*time.Second)-10) + 10
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.ErrorResponse(w, r, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	flusher.Flush()

	time.Sleep(time.Duration(delay) * time.Second)
	s.JSONResponse(w, r, map[string]int{"delay": delay})

	flusher.Flush()
}
