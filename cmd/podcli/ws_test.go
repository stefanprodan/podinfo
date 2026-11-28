package main

import (
	"net"
	"testing"
)

func TestWsConnectError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	if err := wsCmd.RunE(wsCmd, []string{addr + "/ws/echo"}); err == nil {
		t.Error("got no error for a refused connection")
	}
}
