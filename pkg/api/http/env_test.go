package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestEnvHandler(t *testing.T) {
	const marker = "PODINFO_ENV_TEST_MARKER=podinfo-env-test"
	t.Setenv("PODINFO_ENV_TEST_MARKER", "podinfo-env-test")

	req, err := http.NewRequest("GET", "/api/env", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()
	handler := http.HandlerFunc(srv.envHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}

	var envs []string
	if err := json.Unmarshal(rr.Body.Bytes(), &envs); err != nil {
		t.Fatalf("handler returned non-JSON array body: %v\nbody: %s", err, rr.Body.String())
	}

	found := false
	for _, e := range envs {
		if e == marker {
			found = true
			break
		}
	}
	if !found {
		// Fall back to os.Environ shape for the error message.
		t.Fatalf("handler body missing %q; got %d env entries (sample os.Environ len=%d)",
			marker, len(envs), len(os.Environ()))
	}
}
