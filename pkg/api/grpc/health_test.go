// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"
	"net"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

func newBufconnClient(t *testing.T, srv *grpc.Server) *grpc.ClientConn {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		_ = lis.Close()
	})
	t.Cleanup(srv.Stop)

	go func() {
		_ = srv.Serve(lis)
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn
}

func TestGrpcHealth(t *testing.T) {
	const (
		serving    = grpc_health_v1.HealthCheckResponse_SERVING
		notServing = grpc_health_v1.HealthCheckResponse_NOT_SERVING
	)

	tests := []struct {
		name        string
		serviceName string
		unhealthy   bool
		unready     bool
		live        grpc_health_v1.HealthCheckResponse_ServingStatus
		ready       grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{name: "healthy and ready", serviceName: "podinfo", live: serving, ready: serving},
		{name: "unready", serviceName: "podinfo", unready: true, live: serving, ready: notServing},
		{name: "unhealthy", serviceName: "podinfo", unhealthy: true, live: notServing, ready: notServing},
		{name: "unready without service name", unready: true, live: serving, ready: serving},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				logger: zap.NewNop(),
				config: &Config{
					ServiceName: tt.serviceName,
					Unhealthy:   tt.unhealthy,
					Unready:     tt.unready,
				},
			}
			srv, healthSrv := s.newServer()
			client := grpc_health_v1.NewHealthClient(newBufconnClient(t, srv))

			assertHealth(t, client, "", tt.live)
			if tt.serviceName != "" {
				assertHealth(t, client, tt.serviceName, tt.ready)
			}

			healthSrv.Shutdown()

			assertHealth(t, client, "", notServing)
			if tt.serviceName != "" {
				assertHealth(t, client, tt.serviceName, notServing)
			}
		})
	}
}

func assertHealth(t *testing.T, client grpc_health_v1.HealthClient, service string, want grpc_health_v1.HealthCheckResponse_ServingStatus) {
	t.Helper()

	res, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: service})
	if err != nil {
		t.Fatalf("Check(%q) returned error %v", service, err)
	}
	if res.GetStatus() != want {
		t.Errorf("Check(%q) returned %s, want %s", service, res.GetStatus(), want)
	}
}
