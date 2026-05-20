package http

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestEnvHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/env", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()
	handler := http.HandlerFunc(srv.infoHandler)

	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := ".*hostname.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(rr.Body.String()) {
		t.Fatalf("handler returned unexpected body:\ngot \n%v \nwant \n%s",
			rr.Body.String(), expected)
	}
}

func TestEnvHandler_Actual(t *testing.T) {
	srv := NewMockServer()
	req, _ := http.NewRequest("GET", "/env", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.envHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json", rr.Header().Get("Content-Type"))
	}
}
