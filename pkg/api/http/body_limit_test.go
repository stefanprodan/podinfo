package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequestBodySizeLimit verifies body-reading handlers reject payloads larger
// than maxRequestBodySize instead of buffering them into memory. The echo
// handler is used because with no backends configured it simply reflects the
// body and needs no external dependencies.
func TestRequestBodySizeLimit(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/echo", srv.echoHandler)

	// A body within the limit is accepted (202).
	within := httptest.NewRequest("POST", "/echo", bytes.NewReader(make([]byte, 1024)))
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, within)
	if rr.Code != http.StatusAccepted {
		t.Errorf("within-limit body: got status %d want %d", rr.Code, http.StatusAccepted)
	}

	// A body over the limit is rejected with a 413 code in the response body.
	over := httptest.NewRequest("POST", "/echo", bytes.NewReader(make([]byte, maxRequestBodySize+1)))
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
