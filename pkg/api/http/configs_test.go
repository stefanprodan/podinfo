// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stefanprodan/podinfo/pkg/fscache"
	"go.uber.org/zap"
)

func TestConfigReadHandler(t *testing.T) {
	srv := NewMockServer()
	req, _ := http.NewRequest(http.MethodGet, "/configs", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.configReadHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "{}") {
		t.Errorf("expected empty JSON object, got: %s", rr.Body.String())
	}
}

func TestConfigReadHandlerFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.yaml"), []byte("color: blue"), 0o600); err != nil {
		t.Fatal(err)
	}
	configs, err := fscache.New(dir, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = configs.Close() })

	srv := NewMockServer()
	srv.configs = configs
	req, _ := http.NewRequest(http.MethodGet, "/configs", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.configReadHandler).ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), `"app.yaml": "color: blue"`) {
		t.Errorf("expected app.yaml in the response, got: %s", rr.Body.String())
	}
}
