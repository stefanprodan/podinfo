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

type RandomDelayMiddleware struct {
	min  int
	max  int
	unit string
}

func NewRandomDelayMiddleware(minDelay, maxDelay int, delayUnit string) *RandomDelayMiddleware {
	return &RandomDelayMiddleware{
		min:  minDelay,
		max:  maxDelay,
		unit: delayUnit,
	}
}

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
			delay += rand.Intn(m.max - m.min)
		}
		time.Sleep(time.Duration(delay) * unit)
		next.ServeHTTP(w, r)
	})
}

// Delay godoc
// @Summary Delay
// @Description waits for the specified period
// @Tags HTTP API
// @Accept json
// @Produce json
// @Param seconds path int true "seconds to wait for"
// @Router /delay/{seconds} [get]
// @Success 200 {object} http.MapResponse
// @Failure 400 {object} http.ErrorMessage "invalid delay"
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
