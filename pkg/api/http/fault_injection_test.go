// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFaultInjection_EnableDisable(t *testing.T) {
	defer faultInjection.Store(0)
	srv := NewMockServer()

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/fault_injection/enable", nil)
	http.HandlerFunc(srv.enableFaultInjectionHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("enable: got %d want %d", rr.Code, http.StatusAccepted)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("enable: Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rr.Body.String(), `"enabled"`) {
		t.Errorf("enable: expected enabled in body, got: %s", rr.Body.String())
	}
	if faultInjection.Load() != 1 {
		t.Fatalf("faultInjection flag not set after enable")
	}

	rr = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/fault_injection/disable", nil)
	http.HandlerFunc(srv.disableFaultInjectionHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("disable: got %d want %d", rr.Code, http.StatusAccepted)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("disable: Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rr.Body.String(), `"disabled"`) {
		t.Errorf("disable: expected disabled in body, got: %s", rr.Body.String())
	}
	if faultInjection.Load() != 0 {
		t.Fatalf("faultInjection flag not cleared after disable")
	}
}

func TestFaultInjection_StatusHandler(t *testing.T) {
	defer faultInjection.Store(0)
	srv := NewMockServer()

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/fault_injection/status", nil)
	http.HandlerFunc(srv.faultInjectionStatusHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "disabled") {
		t.Errorf("expected disabled in body, got: %s", rr.Body.String())
	}

	faultInjection.Store(1)
	rr = httptest.NewRecorder()
	http.HandlerFunc(srv.faultInjectionStatusHandler).ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "enabled") {
		t.Errorf("expected enabled in body, got: %s", rr.Body.String())
	}
}

func TestFaultInjectionMiddleware(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
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
				faultInjection.Store(1)
			} else {
				faultInjection.Store(0)
			}
			defer faultInjection.Store(0)

			req, _ := http.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tc.wantStatus {
				t.Fatalf("path %s: got %d want %d", tc.path, rr.Code, tc.wantStatus)
			}
		})
	}
}

func TestFaultInjectionMiddleware_Response(t *testing.T) {
	defer faultInjection.Store(0)
	faultInjection.Store(1)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for name, h := range map[string]http.Handler{
		"function": faultInjectionMiddleware(next),
		"method":   NewMockServer().faultInjectionMiddleware(next),
	} {
		t.Run(name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/version", nil))

			if rr.Code != http.StatusInternalServerError {
				t.Errorf("got status %d, want %d", rr.Code, http.StatusInternalServerError)
			}
			if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Errorf("got Content-Type %q, want application/json; charset=utf-8", ct)
			}
			var body map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("body %q is not JSON: %v", rr.Body.String(), err)
			}
			if body["status"] != "fault injection enabled" {
				t.Errorf("got body %v, want status fault injection enabled", body)
			}
		})
	}
}

func TestFaultInjectionExcluded(t *testing.T) {
	cases := map[string]bool{
		"/":                        false,
		"/api/info":                false,
		"/healthz":                 true,
		"/readyz":                  true,
		"/metrics":                 true,
		"/debug/pprof/":            true,
		"/fault_injection/enable":  true,
		"/fault_injection/disable": true,
		"/fault_injection/status":  true,
		"/healthzz":                false,
	}
	for p, want := range cases {
		if got := faultInjectionExcluded(p); got != want {
			t.Errorf("faultInjectionExcluded(%q) = %v, want %v", p, got, want)
		}
	}
}
