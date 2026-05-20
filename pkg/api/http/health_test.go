package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHealthzHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()
	handler := http.HandlerFunc(srv.healthzHandler)

	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusServiceUnavailable)
	}
}

func TestReadyzHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/readyz", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()
	handler := http.HandlerFunc(srv.readyzHandler)

	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusServiceUnavailable)
	}
}

func TestHealthzHandler_Healthy(t *testing.T) {
	srv := NewMockServer()
	atomic.StoreInt32(&healthy, 1)
	defer atomic.StoreInt32(&healthy, 0)

	req, _ := http.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.healthzHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "OK") {
		t.Errorf("expected OK in body, got: %s", rr.Body.String())
	}
}

func TestReadyzHandler_Ready(t *testing.T) {
	srv := NewMockServer()
	atomic.StoreInt32(&ready, 1)
	defer atomic.StoreInt32(&ready, 0)

	req, _ := http.NewRequest("GET", "/readyz", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.readyzHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestEnableReadyHandler(t *testing.T) {
	srv := NewMockServer()
	atomic.StoreInt32(&ready, 0)

	req, _ := http.NewRequest("POST", "/readyz/enable", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.enableReadyHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusAccepted)
	}
	if atomic.LoadInt32(&ready) != 1 {
		t.Error("ready flag not set to 1")
	}
	atomic.StoreInt32(&ready, 0)
}

func TestDisableReadyHandler(t *testing.T) {
	srv := NewMockServer()
	atomic.StoreInt32(&ready, 1)

	req, _ := http.NewRequest("POST", "/readyz/disable", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.disableReadyHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusAccepted)
	}
	if atomic.LoadInt32(&ready) != 0 {
		t.Error("ready flag not set to 0")
	}
}
