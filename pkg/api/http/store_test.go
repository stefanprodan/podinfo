package http

import (
	"encoding/json"
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

func TestStoreWriteHandler(t *testing.T) {
	srv := NewMockServer()
	srv.config.DataPath = t.TempDir()

	payload := "hello world"
	req, _ := http.NewRequest("POST", "/store", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.storeWriteHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusAccepted)
	}

	var result map[string]string
	json.Unmarshal(rr.Body.Bytes(), &result)
	if result["hash"] != hash(payload) {
		t.Errorf("hash = %q, want %q", result["hash"], hash(payload))
	}
}

func TestStoreWriteHandler_InvalidPath(t *testing.T) {
	srv := NewMockServer()
	srv.config.DataPath = "/nonexistent/path/that/does/not/exist"

	req, _ := http.NewRequest("POST", "/store", strings.NewReader("data"))
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.storeWriteHandler).ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "writing file failed") {
		t.Errorf("expected write error, got: %s", rr.Body.String())
	}
}

func TestStoreReadHandler_NotFound(t *testing.T) {
	srv := NewMockServer()
	srv.config.DataPath = t.TempDir()
	srv.router.HandleFunc("/store/{hash}", srv.storeReadHandler)

	req, _ := http.NewRequest("GET", "/store/aabbccddee11223344556677889900aabbccddee", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "reading file failed") {
		t.Errorf("expected read error, got: %s", rr.Body.String())
	}
}
