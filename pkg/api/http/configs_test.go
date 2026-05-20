package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigReadHandler(t *testing.T) {
	srv := NewMockServer()
	req, _ := http.NewRequest("GET", "/configs", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.configReadHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "{}") {
		t.Errorf("expected empty JSON object, got: %s", rr.Body.String())
	}
}
