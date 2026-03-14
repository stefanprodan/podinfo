package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestStoreReadHandler_ContentType(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewMockServer()
	srv.config.DataPath = dataDir

	// Write an HTML payload to the store.
	writeReq, err := http.NewRequest("POST", "/store", strings.NewReader("<html><script>alert(1)</script></html>"))
	if err != nil {
		t.Fatal(err)
	}
	writeRR := httptest.NewRecorder()
	http.HandlerFunc(srv.storeWriteHandler).ServeHTTP(writeRR, writeReq)

	if writeRR.Code != http.StatusAccepted {
		t.Fatalf("store write returned status %d, want %d", writeRR.Code, http.StatusAccepted)
	}

	// Read it back and verify Content-Type is application/octet-stream, not text/html.
	hash := hash("<html><script>alert(1)</script></html>")
	readReq, err := http.NewRequest("GET", "/store/"+hash, nil)
	if err != nil {
		t.Fatal(err)
	}
	readReq = mux.SetURLVars(readReq, map[string]string{"hash": hash})

	readRR := httptest.NewRecorder()
	http.HandlerFunc(srv.storeReadHandler).ServeHTTP(readRR, readReq)

	if readRR.Code != http.StatusAccepted {
		t.Fatalf("store read returned status %d, want %d", readRR.Code, http.StatusAccepted)
	}

	expectedHeaders := map[string]string{
		"Content-Type":            "application/octet-stream",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "default-src 'none'",
	}
	for header, want := range expectedHeaders {
		if got := readRR.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestStoreReadHandler_PathTraversal(t *testing.T) {
	srv := NewMockServer()
	srv.config.DataPath = t.TempDir()

	traversalPaths := []string{
		"../../../../etc/passwd",
		"../../../etc/shadow",
		"..%2f..%2f..%2fetc%2fpasswd",
		"abc123",
		"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzg", // 40 chars but not hex
	}

	for _, tp := range traversalPaths {
		req, err := http.NewRequest("GET", "/store/"+tp, nil)
		if err != nil {
			t.Fatal(err)
		}
		req = mux.SetURLVars(req, map[string]string{"hash": tp})

		rr := httptest.NewRecorder()
		http.HandlerFunc(srv.storeReadHandler).ServeHTTP(rr, req)

		if !strings.Contains(rr.Body.String(), "invalid hash") {
			t.Errorf("path %q: expected 'invalid hash' error, got %q", tp, rr.Body.String())
		}
	}
}
