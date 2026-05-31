package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestFaultInjection_EnableDisable(t *testing.T) {
	defer atomic.StoreInt32(&faultInjection, 0)
	srv := NewMockServer()

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/fault_injection/enable", nil)
	http.HandlerFunc(srv.enableFaultInjectionHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("enable: got %d want %d", rr.Code, http.StatusAccepted)
	}
	if atomic.LoadInt32(&faultInjection) != 1 {
		t.Fatalf("faultInjection flag not set after enable")
	}

	rr = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/fault_injection/disable", nil)
	http.HandlerFunc(srv.disableFaultInjectionHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("disable: got %d want %d", rr.Code, http.StatusAccepted)
	}
	if atomic.LoadInt32(&faultInjection) != 0 {
		t.Fatalf("faultInjection flag not cleared after disable")
	}
}

func TestFaultInjection_StatusHandler(t *testing.T) {
	defer atomic.StoreInt32(&faultInjection, 0)
	srv := NewMockServer()

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/fault_injection/status", nil)
	http.HandlerFunc(srv.faultInjectionStatusHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "disabled") {
		t.Errorf("expected disabled in body, got: %s", rr.Body.String())
	}

	atomic.StoreInt32(&faultInjection, 1)
	rr = httptest.NewRecorder()
	http.HandlerFunc(srv.faultInjectionStatusHandler).ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "enabled") {
		t.Errorf("expected enabled in body, got: %s", rr.Body.String())
	}
}

func TestFaultInjectionMiddleware(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	h := faultInjectionMiddleware(ok)

	cases := []struct {
		name       string
		path       string
		injected   bool
		wantStatus int
	}{
		{"disabled passes through", "/", false, http.StatusOK},
		{"enabled returns 500 for app path", "/", true, http.StatusInternalServerError},
		{"enabled returns 500 for arbitrary path", "/api/info", true, http.StatusInternalServerError},
		{"enabled excludes healthz", "/healthz", true, http.StatusOK},
		{"enabled excludes readyz", "/readyz", true, http.StatusOK},
		{"enabled excludes metrics", "/metrics", true, http.StatusOK},
		{"enabled excludes debug pprof", "/debug/pprof/", true, http.StatusOK},
		{"enabled excludes fault_injection control", "/fault_injection/disable", true, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.injected {
				atomic.StoreInt32(&faultInjection, 1)
			} else {
				atomic.StoreInt32(&faultInjection, 0)
			}
			defer atomic.StoreInt32(&faultInjection, 0)

			req, _ := http.NewRequest("GET", tc.path, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tc.wantStatus {
				t.Fatalf("path %s: got %d want %d", tc.path, rr.Code, tc.wantStatus)
			}
		})
	}
}

func TestFaultInjectionExcluded(t *testing.T) {
	cases := map[string]bool{
		"/":                          false,
		"/api/info":                  false,
		"/healthz":                   true,
		"/readyz":                    true,
		"/metrics":                   true,
		"/debug/pprof/":              true,
		"/fault_injection/enable":    true,
		"/fault_injection/disable":   true,
		"/fault_injection/status":    true,
		"/healthzz":                  false,
	}
	for p, want := range cases {
		if got := faultInjectionExcluded(p); got != want {
			t.Errorf("faultInjectionExcluded(%q) = %v, want %v", p, got, want)
		}
	}
}
