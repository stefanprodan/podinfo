package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDebugEndpoints(t *testing.T) {
	srv := NewMockServer()
	srv.registerHandlers()

	cases := map[string]int{
		"/debug/pprof/":                  http.StatusOK,
		"/debug/pprof/goroutine?debug=1": http.StatusOK,
		"/debug/pprof/symbol":            http.StatusOK,
		"/debug/pprof/cmdline":           http.StatusNotFound,
		"/debug/pprof/cmdline?debug=1":   http.StatusNotFound,
		"/debug/vars":                    http.StatusNotFound,
		"/debug/requests":                http.StatusNotFound,
	}
	ports := map[string]http.Handler{"main": srv.router, "metrics": srv.metricsServer().Handler}
	for port, h := range ports {
		for p, want := range cases {
			req := httptest.NewRequest("GET", p, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != want {
				t.Errorf("%s port GET %s: got %d want %d", port, p, rr.Code, want)
			}
		}
	}
}

func TestMetricsServerAddr(t *testing.T) {
	srv := NewMockServer()
	srv.config.Host = "127.0.0.1"
	srv.config.PortMetrics = 9797
	if got, want := srv.metricsServer().Addr, "127.0.0.1:9797"; got != want {
		t.Errorf("metrics server Addr = %q, want %q", got, want)
	}
}
