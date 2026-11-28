package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequestBodySizeLimit exercises the limit through the echo handler, which
// has no dependencies when no backend is set.
func TestRequestBodySizeLimit(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/echo", srv.echoHandler)

	within := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(make([]byte, 1024)))
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, within)
	if rr.Code != http.StatusAccepted {
		t.Errorf("within-limit body: got status %d want %d", rr.Code, http.StatusAccepted)
	}

	over := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(make([]byte, maxRequestBodySize+1)))
	rr = httptest.NewRecorder()
	srv.router.ServeHTTP(rr, over)

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("oversize body: response is not the expected error JSON: %v (body=%q)", err, rr.Body.String())
	}
	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize body: got code %d want %d", resp.Code, http.StatusRequestEntityTooLarge)
	}
}
