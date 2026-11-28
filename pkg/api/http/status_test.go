package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestStatusHandler(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/status/404", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()

	srv.router.HandleFunc("/status/{code}", srv.statusHandler)
	srv.router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestStatusHandler_Various(t *testing.T) {
	logs, observed := observer.New(zapcore.ErrorLevel)
	srv := NewMockServer()
	srv.logger = zap.New(logs)
	srv.router.HandleFunc("/status/{code}", srv.statusHandler)
	ts := httptest.NewServer(srv.router)
	defer ts.Close()

	cases := []struct {
		code     string
		expected int
		body     string
	}{
		{"99", 400, "invalid status code"},
		{"100", 400, "invalid status code"},
		{"101", 400, "invalid status code"},
		{"102", 400, "invalid status code"},
		{"199", 400, "invalid status code"},
		{"200", 200, `"status": 200`},
		{"201", 201, `"status": 201`},
		{"204", 204, ""},
		{"304", 304, ""},
		{"404", 404, `"status": 404`},
		{"500", 500, `"status": 500`},
		{"503", 503, `"status": 503`},
		{"999", 999, `"status": 999`},
		{"1000", 400, "invalid status code"},
	}

	for _, c := range cases {
		resp, err := ts.Client().Get(ts.URL + "/status/" + c.code)
		if err != nil {
			t.Fatalf("/status/%s: %v", c.code, err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("/status/%s: %v", c.code, err)
		}

		if resp.StatusCode != c.expected {
			t.Errorf("/status/%s: got %d, want %d", c.code, resp.StatusCode, c.expected)
		}
		if c.body == "" && len(body) != 0 {
			t.Errorf("/status/%s: got body %q, want none", c.code, body)
		}
		if !strings.Contains(string(body), c.body) {
			t.Errorf("/status/%s: got body %q, want %q", c.code, body, c.body)
		}
	}
	if n := observed.Len(); n != 0 {
		t.Errorf("got %d errors: %v", n, observed.All())
	}
}
