package http

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestDelayHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/delay/0", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()

	srv.router.HandleFunc("/delay/{wait}", srv.delayHandler)
	srv.router.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := ".*delay.*0.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(rr.Body.String()) {
		t.Fatalf("handler returned unexpected body:\ngot \n%v \nwant \n%s",
			rr.Body.String(), expected)
	}
}

func TestRandomDelayMiddleware(t *testing.T) {
	m := NewRandomDelayMiddleware(0, 1, "ms")
	called := false
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("next handler was not called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestRandomDelayMiddleware_Milliseconds(t *testing.T) {
	m := NewRandomDelayMiddleware(0, 1, "ms")
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestRandomErrorMiddleware(t *testing.T) {
	handler := randomErrorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	gotOK := false
	gotError := false
	for i := 0; i < 50; i++ {
		req, _ := http.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code == http.StatusOK {
			gotOK = true
		} else {
			gotError = true
		}
		if gotOK && gotError {
			break
		}
	}
	if !gotOK {
		t.Error("never got a successful response")
	}
	if !gotError {
		t.Error("never got an error response")
	}
}
