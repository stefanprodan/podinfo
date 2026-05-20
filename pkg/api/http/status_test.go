package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/status/404", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()

	srv.router.HandleFunc("/status/{code}", srv.statusHandler)
	srv.router.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestStatusHandler_Various(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/status/{code}", srv.statusHandler)

	cases := []struct {
		code     string
		expected int
	}{
		{"200", 200},
		{"201", 201},
		{"500", 500},
		{"503", 503},
	}

	for _, c := range cases {
		req, _ := http.NewRequest("GET", "/status/"+c.code, nil)
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != c.expected {
			t.Errorf("/status/%s: got %d, want %d", c.code, rr.Code, c.expected)
		}
	}
}
