package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTokenHandler(t *testing.T) {
	req, err := http.NewRequest("POST", "/token", strings.NewReader("test-user"))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()
	handler := http.HandlerFunc(srv.tokenGenerateHandler)

	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var token TokenResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &token); err != nil {
		t.Fatal(err)
	}
	if token.Token == "" {
		t.Error("handler returned no token")
	}
}
