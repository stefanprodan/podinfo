package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPprofHandler(t *testing.T) {
	h := pprofHandler()

	cases := map[string]int{
		"/debug/pprof/":                  http.StatusOK,
		"/debug/pprof/goroutine?debug=1": http.StatusOK,
		"/debug/pprof/symbol":            http.StatusOK,
		"/debug/pprof/cmdline":           http.StatusNotFound,
		"/debug/pprof/cmdline?debug=1":   http.StatusNotFound,
	}
	for p, want := range cases {
		req := httptest.NewRequest("GET", p, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != want {
			t.Errorf("GET %s: got %d want %d", p, rr.Code, want)
		}
	}
}
