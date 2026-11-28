// Copyright 2018 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// RandomDelayMiddleware delays every request by a random duration between
// min and max, in seconds or milliseconds.
type RandomDelayMiddleware struct {
	min  int
	max  int
	unit string
}

// NewRandomDelayMiddleware returns a middleware that sleeps between minDelay
// and maxDelay. delayUnit is "s" or "ms", any other value counts as seconds.
func NewRandomDelayMiddleware(minDelay, maxDelay int, delayUnit string) *RandomDelayMiddleware {
	return &RandomDelayMiddleware{
		min:  minDelay,
		max:  maxDelay,
		unit: delayUnit,
	}
}

// Handler returns next wrapped so that every request sleeps first.
func (m *RandomDelayMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var unit time.Duration
		switch m.unit {
		case "s":
			unit = time.Second
		case "ms":
			unit = time.Millisecond
		default:
			unit = time.Second
		}

		delay := m.min
		if m.max > m.min {
			delay += rand.Intn(m.max - m.min) //nolint:gosec // the delay needs no secure randomness
		}
		time.Sleep(time.Duration(delay) * unit)
		next.ServeHTTP(w, r)
	})
}

// Delay godoc
// @Summary Delay
// @Description waits for the specified period
// @Tags HTTP API
// @Produce json
// @Param seconds path int true "seconds to wait for"
// @Router /delay/{seconds} [get]
// @Success 200 {object} map[string]int "the delay in seconds"
// @Failure 400 {object} http.ErrorMessage "invalid delay or request body"
// @Failure 413 {object} http.ErrorMessage "request body too large"
// @Failure 499 {object} http.ErrorMessage "client closed request"
func (s *Server) delayHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	delay, err := strconv.Atoi(vars["wait"])
	if err != nil {
		s.ErrorResponse(w, r, err.Error(), http.StatusBadRequest)
		return
	}

	if !s.drainBody(w, r) {
		return
	}
	if !wait(r.Context(), delay) {
		s.ErrorResponse(w, r, "client closed request", statusClientClosedRequest)
		return
	}

	s.JSONResponse(w, r, map[string]int{"delay": delay})
}

// drainBody reads the request body to the end, so the server notices a
// client disconnect while the handler waits.
func (s *Server) drainBody(w http.ResponseWriter, r *http.Request) bool {
	if r.Body == nil {
		return true
	}
	_, err := io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, 1<<20))
	if err == nil {
		return true
	}
	code := http.StatusBadRequest
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		code = http.StatusRequestEntityTooLarge
	}
	s.ErrorResponse(w, r, err.Error(), code)
	return false
}

// wait sleeps for the given seconds and reports whether they elapsed before
// ctx was done.
func wait(ctx context.Context, seconds int) bool {
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
