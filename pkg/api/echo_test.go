package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEchoHandler(t *testing.T) {
	expected := `{"test": true}`
	req, err := http.NewRequest("POST", "/api/echo", strings.NewReader(expected))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()
	handler := http.HandlerFunc(srv.echoHandler)

	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusAccepted {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusAccepted)
	}

	// Check the response body is what we expect.
	if rr.Body.String() != expected {
		t.Fatalf("handler returned unexpected body:\ngot \n%v \nwant \n%s",
			rr.Body.String(), expected)
	}
}
