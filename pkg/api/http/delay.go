package http

import (
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
		rand.Seed(time.Now().Unix())
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
func (s *Server) delayHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "delayHandler")
	defer span.End()

	vars := mux.Vars(r)

	delay, err := strconv.Atoi(vars["wait"])
	if err != nil {
		s.ErrorResponse(w, r, span, err.Error(), http.StatusBadRequest)
		return
	}

	time.Sleep(time.Duration(delay) * time.Second)

	s.JSONResponse(w, r, map[string]int{"delay": delay})
}
