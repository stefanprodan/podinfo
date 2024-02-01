package api

import (
	"math/rand"
	"net/http"
	"regexp"

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
// @Success 200 {object} api.MapResponse
func (s *Server) delayHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "delayHandler")
	defer span.End()

	vars := mux.Vars(r)

	wait := vars["wait"]
	re := regexp.MustCompile(`(?P<duration>\d+)(?P<unit>ms|s)?`)

	match := re.FindStringSubmatch(wait)
	delay, err := strconv.Atoi(match[1])

	unit := time.Second
	unitString := "s"
	if match[2] == "ms" {
		unit = time.Millisecond
		unitString = "ms"
	}

	if err != nil {
		s.ErrorResponse(w, r, span, err.Error(), http.StatusBadRequest)
		return
	}

	time.Sleep(time.Duration(delay) * unit)

	s.JSONResponse(w, r, map[string]any{"delay": delay, "unit": unitString})
}
