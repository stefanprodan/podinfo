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
