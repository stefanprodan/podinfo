package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizePrefix(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{name: "default root", input: "/", output: "/"},
		{name: "empty", input: "", output: "/"},
		{name: "no leading slash", input: "foo", output: "/foo"},
		{name: "trailing slash", input: "/foo/", output: "/foo"},
		{name: "double slash", input: "//foo//bar//", output: "/foo/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePrefix(tt.input); got != tt.output {
				t.Fatalf("normalizePrefix(%q) = %q, want %q", tt.input, got, tt.output)
			}
		})
	}
}

func TestRegisterHandlersWithPrefix(t *testing.T) {
	srv := NewMockServer()
	srv.config.Prefix = normalizePrefix("/foo")
	srv.registerHandlers()

	reqRootSlash, err := http.NewRequest("GET", "/foo/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rrRootSlash := httptest.NewRecorder()
	srv.router.ServeHTTP(rrRootSlash, reqRootSlash)

	if rrRootSlash.Code != http.StatusOK {
		t.Fatalf("GET /foo/ returned %d, want %d", rrRootSlash.Code, http.StatusOK)
	}

	req, err := http.NewRequest("GET", "/foo/api/info", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /foo/api/info returned %d, want %d", rr.Code, http.StatusOK)
	}

	reqNoPrefix, err := http.NewRequest("GET", "/api/info", nil)
	if err != nil {
		t.Fatal(err)
	}
	rrNoPrefix := httptest.NewRecorder()
	srv.router.ServeHTTP(rrNoPrefix, reqNoPrefix)

	if rrNoPrefix.Code != http.StatusNotFound {
		t.Fatalf("GET /api/info returned %d, want %d", rrNoPrefix.Code, http.StatusNotFound)
	}
}
