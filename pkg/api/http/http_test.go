package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionMiddleware(t *testing.T) {
	srv := NewMockServer()
	handler := versionMiddleware(http.HandlerFunc(srv.infoHandler))

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if req.Header.Get("X-API-Version") == "" {
		t.Error("X-API-Version not set by middleware")
	}
	if req.Header.Get("X-API-Revision") == "" {
		t.Error("X-API-Revision not set by middleware")
	}
}
