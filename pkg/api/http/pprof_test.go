// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestDebugEndpoints(t *testing.T) {
	srv := NewMockServer()
	srv.registerHandlers()

	cases := map[string]int{
		"/debug/pprof/":                  http.StatusOK,
		"/debug/pprof/goroutine?debug=1": http.StatusOK,
		"/debug/pprof/symbol":            http.StatusOK,
		"/debug/pprof/cmdline":           http.StatusNotFound,
		"/debug/pprof/cmdline?debug=1":   http.StatusNotFound,
		"/debug/vars":                    http.StatusNotFound,
		"/debug/requests":                http.StatusNotFound,
	}
	ports := map[string]http.Handler{"main": srv.router, "metrics": srv.metricsServer().Handler}
	for port, h := range ports {
		for p, want := range cases {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != want {
				t.Errorf("%s port GET %s: got %d want %d", port, p, rr.Code, want)
			}
		}
	}
}

func TestMetricsServerAddr(t *testing.T) {
	srv := NewMockServer()
	srv.config.Host = "127.0.0.1"
	srv.config.PortMetrics = 9797
	if got, want := srv.metricsServer().Addr, "127.0.0.1:9797"; got != want {
		t.Errorf("metrics server Addr = %q, want %q", got, want)
	}
}

func TestMetricsServerTimeout(t *testing.T) {
	srv := NewMockServer()
	if got, want := srv.metricsServer().ReadHeaderTimeout, srv.config.HTTPServerTimeout; got != want {
		t.Errorf("metrics server ReadHeaderTimeout = %s, want %s", got, want)
	}
}

func TestMetricsServerBindError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	core, logs := observer.New(zap.ErrorLevel)
	srv := NewMockServer()
	srv.logger = zap.New(core, zap.WithFatalHook(zapcore.WriteThenGoexit))
	srv.config.Host = "127.0.0.1"
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is %T, want *net.TCPAddr", ln.Addr())
	}
	srv.config.PortMetrics = addr.Port

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.startMetricsServer()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the metrics server did not stop on a bind error")
	}
	entries := logs.FilterMessage("metrics server crashed").All()
	if len(entries) != 1 || entries[0].Level != zap.FatalLevel {
		t.Fatalf("got log entries %v, want one fatal entry", logs.All())
	}
}
