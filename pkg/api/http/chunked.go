package http

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
// @Param seconds path int true "seconds to wait for"
// @Router /chunked/{seconds} [get]
// @Success 200 {object} http.MapResponse
func (s *Server) chunkedHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "chunkedHandler")
	defer span.End()

	vars := mux.Vars(r)

	delay, err := strconv.Atoi(vars["wait"])
	if err != nil {
		delay = randomDelaySeconds(s.config.HttpServerTimeout)
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.ErrorResponse(w, r, span, "Streaming unsupported!", http.StatusInternalServerError)
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

// randomDelaySeconds returns a random delay in seconds within [10, timeout),
// used when no explicit wait is provided. timeout is a time.Duration, so it is
// converted to whole seconds; the upper bound is clamped to keep rand.Intn's
// argument positive (it panics on a non-positive argument).
func randomDelaySeconds(timeout time.Duration) int {
	maxDelay := int(timeout / time.Second)
	if maxDelay <= 11 {
		maxDelay = 12
	}
	return rand.Intn(maxDelay-10) + 10
}
