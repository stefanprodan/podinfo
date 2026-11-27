package http

import (
	"context"
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

		delay := rand.Intn(m.max-m.min) + m.min
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

	if !wait(r.Context(), delay) {
		return
	}

	s.JSONResponse(w, r, map[string]int{"delay": delay})
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
