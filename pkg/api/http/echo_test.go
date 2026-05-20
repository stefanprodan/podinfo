package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEchoHandler(t *testing.T) {
	cases := []struct {
		url      string
		method   string
		expected string
	}{
		{url: "/api/echo", method: "POST", expected: `{"test": true}`},
		{url: "/api/echo", method: "PUT", expected: `{"test": true}`},
		{url: "/echo", method: "PUT", expected: `{"test": true}`},
		{url: "/echo/", method: "POST", expected: `{"test": true}`},
		{url: "/echo/test", method: "POST", expected: `{"test": true}`},
		{url: "/echo/test/", method: "POST", expected: `{"test": true}`},
		{url: "/echo/test/test123-test", method: "POST", expected: `{"test": true}`},
	}

	srv := NewMockServer()
	handler := http.HandlerFunc(srv.echoHandler)

	for _, c := range cases {
		req, err := http.NewRequest(c.method, c.url, strings.NewReader(c.expected))
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		// Check the status code is what we expect.
		if status := rr.Code; status != http.StatusAccepted {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusAccepted)
		}

		// Check the response body is what we expect.
		if rr.Body.String() != c.expected {
			t.Fatalf("handler returned unexpected body:\ngot \n%v \nwant \n%s",
				rr.Body.String(), c.expected)
		}
	}
}

func TestEchoHandler_ContentType(t *testing.T) {
	srv := NewMockServer()
	handler := http.HandlerFunc(srv.echoHandler)

	payload := "<html><script>alert(1)</script></html>"
	req, err := http.NewRequest("POST", "/echo", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("echo returned status %d, want %d", rr.Code, http.StatusAccepted)
	}

	expectedHeaders := map[string]string{
		"Content-Type":            "application/octet-stream",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "default-src 'none'",
	}
	for header, want := range expectedHeaders {
		if got := rr.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestCopyTracingHeaders(t *testing.T) {
	from, _ := http.NewRequest("GET", "/", nil)
	from.Header.Set("x-request-id", "abc123")
	from.Header.Set("x-b3-traceid", "trace-1")
	from.Header.Set("x-b3-spanid", "span-1")

	to, _ := http.NewRequest("POST", "/echo", nil)
	copyTracingHeaders(from, to)

	if to.Header.Get("x-request-id") != "abc123" {
		t.Errorf("x-request-id not copied")
	}
	if to.Header.Get("x-b3-traceid") != "trace-1" {
		t.Errorf("x-b3-traceid not copied")
	}
	if to.Header.Get("x-b3-spanid") != "span-1" {
		t.Errorf("x-b3-spanid not copied")
	}
}

func TestEchoHandler_WithBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`"backend response"`))
	}))
	defer backend.Close()

	srv := NewMockServer()
	srv.config.BackendURL = []string{backend.URL}

	req, _ := http.NewRequest("POST", "/echo", strings.NewReader("hello"))
	req.Header.Set("x-request-id", "test-123")
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.echoHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "backend response") {
		t.Errorf("expected backend response in body, got: %s", rr.Body.String())
	}
	if rr.Header().Get("X-Color") != "blue" {
		t.Errorf("X-Color = %q, want %q", rr.Header().Get("X-Color"), "blue")
	}
}

func TestEchoHandler_BackendError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	srv := NewMockServer()
	srv.config.BackendURL = []string{backend.URL}

	req, _ := http.NewRequest("POST", "/echo", strings.NewReader("hello"))
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.echoHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "response status code 500") {
		t.Errorf("expected error message in body, got: %s", rr.Body.String())
	}
}

func TestEchoHandler_BackendUnreachable(t *testing.T) {
	srv := NewMockServer()
	srv.config.BackendURL = []string{"http://127.0.0.1:1"}

	req, _ := http.NewRequest("POST", "/echo", strings.NewReader("hello"))
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.echoHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "call failed") {
		t.Errorf("expected call failed in body, got: %s", rr.Body.String())
	}
}

func TestEchoHandler_MultipleBackends(t *testing.T) {
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`"resp1"`))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`"resp2"`))
	}))
	defer backend2.Close()

	srv := NewMockServer()
	srv.config.BackendURL = []string{backend1.URL, backend2.URL}

	req, _ := http.NewRequest("POST", "/echo", strings.NewReader("hello"))
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.echoHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "resp1") || !strings.Contains(rr.Body.String(), "resp2") {
		t.Errorf("expected both backend responses, got: %s", rr.Body.String())
	}
}
