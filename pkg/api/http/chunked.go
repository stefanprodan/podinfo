// Copyright 2019 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

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
// @Description sends the response headers with transfer-encoding chunked, waits for the specified period, then sends the body
// @Tags HTTP API
// @Produce json
// @Param seconds path int true "seconds to wait for"
// @Router /chunked/{seconds} [get]
// @Success 200 {object} map[string]int "the delay in seconds"
// @Failure 400 {object} http.ErrorMessage "invalid request body"
// @Failure 413 {object} http.ErrorMessage "request body too large"
// @Failure 500 {object} http.ErrorMessage "Streaming unsupported!"
func (s *Server) chunkedHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	delay, err := strconv.Atoi(vars["wait"])
	if err != nil {
		delay = randomDelaySeconds(s.config.HTTPServerTimeout)
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.ErrorResponse(w, r, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	if !s.drainBody(w, r) {
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	flusher.Flush()

	if !wait(r.Context(), delay) {
		return
	}

	s.JSONResponse(w, r, map[string]int{"delay": delay})

	flusher.Flush()
}

// randomDelaySeconds returns a random delay in [10, timeout) seconds. Timeouts
// under 12s count as 12s, as rand.Intn panics on a non-positive argument.
func randomDelaySeconds(timeout time.Duration) int {
	maxDelay := int(timeout / time.Second)
	if maxDelay <= 11 {
		maxDelay = 12
	}
	return rand.Intn(maxDelay-10) + 10 //nolint:gosec // the delay needs no secure randomness
}
