// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package signals

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	delayv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/delay/v1"
)

type blockingDelayServer struct {
	delayv1.UnimplementedDelayServiceServer
}

func (blockingDelayServer) Delay(ctx context.Context, _ *delayv1.DelayRequest) (*delayv1.DelayResponse, error) {
	<-ctx.Done()
	return nil, status.FromContextError(ctx.Err()).Err()
}

func TestGraceful(t *testing.T) {
	const (
		drainDelay      = time.Minute
		shutdownTimeout = 200 * time.Millisecond
		hangTimeout     = 30 * time.Second
	)

	delayStarted := make(chan struct{})
	var delayOnce sync.Once
	interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasSuffix(info.FullMethod, "/Delay") {
			delayOnce.Do(func() { close(delayStarted) })
		}
		return handler(ctx, req)
	}

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		_ = lis.Close()
	})
	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	delayv1.RegisterDelayServiceServer(grpcSrv, blockingDelayServer{})
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("podinfo", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcSrv, healthSrv)
	go func() {
		_ = grpcSrv.Serve(lis)
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
	healthClient := grpc_health_v1.NewHealthClient(conn)

	httpLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen %v", err)
	}
	httpStarted := make(chan struct{})
	httpShutdown := make(chan struct{})
	httpSrv := &http.Server{ReadHeaderTimeout: time.Minute, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(httpStarted)
		<-httpShutdown
		_, _ = io.WriteString(w, "ok")
	})}
	httpSrv.RegisterOnShutdown(func() { close(httpShutdown) })
	go func() {
		_ = httpSrv.Serve(httpLis)
	}()

	delayErr := make(chan error, 1)
	go func() {
		_, err := delayv1.NewDelayServiceClient(conn).Delay(context.Background(), &delayv1.DelayRequest{Seconds: 3600})
		delayErr <- err
	}()
	httpResult := make(chan error, 1)
	go func() {
		res, err := http.Get("http://" + httpLis.Addr().String())
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode != http.StatusOK {
				err = io.ErrUnexpectedEOF
			}
		}
		httpResult <- err
	}()

	select {
	case <-delayStarted:
	case <-time.After(hangTimeout):
		t.Fatal("Delay call did not reach the server")
	}
	select {
	case <-httpStarted:
	case <-time.After(hangTimeout):
		t.Fatal("HTTP request did not reach the server")
	}

	sd := NewShutdown(shutdownTimeout, drainDelay, zap.NewNop())
	draining := make(chan time.Duration)
	drained := make(chan struct{})
	sd.sleep = func(d time.Duration) {
		draining <- d
		<-drained
	}
	stopCh := make(chan struct{})
	close(stopCh)
	var healthy, ready int32 = 1, 1

	done := make(chan struct{})
	go func() {
		sd.Graceful(stopCh, httpSrv, nil, grpcSrv, healthSrv, &healthy, &ready)
		close(done)
	}()

	select {
	case d := <-draining:
		if d != drainDelay {
			t.Errorf("Graceful drains for %s, want %s", d, drainDelay)
		}
	case <-time.After(hangTimeout):
		t.Fatal("Graceful did not start draining")
	}
	if atomic.LoadInt32(&healthy) != 0 || atomic.LoadInt32(&ready) != 0 {
		t.Errorf("healthy=%d ready=%d, want 0", atomic.LoadInt32(&healthy), atomic.LoadInt32(&ready))
	}
	checkNotServing(t, healthClient, "")
	checkNotServing(t, healthClient, "podinfo")
	select {
	case <-httpShutdown:
		t.Error("HTTP server shut down before the drain delay")
	case err := <-delayErr:
		t.Errorf("in-flight Delay returned before the drain delay: %v", err)
	default:
	}

	start := time.Now()
	close(drained)
	select {
	case <-done:
	case <-time.After(hangTimeout):
		t.Fatal("Graceful did not return")
	}
	if elapsed := time.Since(start); elapsed < shutdownTimeout {
		t.Errorf("Graceful returned %s after the drain delay, want >= %s", elapsed, shutdownTimeout)
	}

	select {
	case err := <-delayErr:
		if status.Code(err) != codes.Unavailable {
			t.Errorf("Delay returned %s, want %s", status.Code(err), codes.Unavailable)
		}
	case <-time.After(hangTimeout):
		t.Error("in-flight Delay was not interrupted")
	}

	select {
	case err := <-httpResult:
		if err != nil {
			t.Errorf("in-flight HTTP request failed: %v", err)
		}
	case <-time.After(hangTimeout):
		t.Error("in-flight HTTP request did not complete")
	}
}

func checkNotServing(t *testing.T, client grpc_health_v1.HealthClient, service string) {
	t.Helper()

	res, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: service})
	if err != nil {
		t.Fatalf("Check(%q) %v", service, err)
	}
	if res.GetStatus() != grpc_health_v1.HealthCheckResponse_NOT_SERVING {
		t.Errorf("Check(%q) is %s, want NOT_SERVING", service, res.GetStatus())
	}
}
