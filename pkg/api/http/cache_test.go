package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCacheWriteHandler_NoPool(t *testing.T) {
	srv := NewMockServer()
	srv.pool = nil
	srv.router.HandleFunc("/cache/{key}", srv.cacheWriteHandler)

	req, _ := http.NewRequest("POST", "/cache/key1", strings.NewReader("value1"))
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server is offline") {
		t.Errorf("expected offline error, got: %s", rr.Body.String())
	}
}

func TestCacheDeleteHandler_NoPool(t *testing.T) {
	srv := NewMockServer()
	srv.pool = nil
	srv.router.HandleFunc("/cache/{key}", srv.cacheDeleteHandler).Methods("DELETE")

	req, _ := http.NewRequest("DELETE", "/cache/key1", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server is offline") {
		t.Errorf("expected offline error, got: %s", rr.Body.String())
	}
}

func TestCacheReadHandler_NoPool(t *testing.T) {
	srv := NewMockServer()
	srv.pool = nil
	srv.router.HandleFunc("/cache/{key}", srv.cacheReadHandler).Methods("GET")

	req, _ := http.NewRequest("GET", "/cache/key1", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server is offline") {
		t.Errorf("expected offline error, got: %s", rr.Body.String())
	}
}
