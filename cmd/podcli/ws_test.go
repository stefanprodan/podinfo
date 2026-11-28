package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWsConnectError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	t.Cleanup(func() { origin = "" })

	if err := wsCmd.RunE(wsCmd, []string{strings.TrimPrefix(srv.URL, "http://") + "/ws/echo"}); err == nil {
		t.Error("got no error for a rejected handshake")
	}
}
