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

	podinfogrpc "github.com/stefanprodan/podinfo/pkg/api/grpc"
	delayv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/delay/v1"
)

func TestGraceful(t *testing.T) {
	const (
		drainDelay      = 500 * time.Millisecond
		shutdownTimeout = 500 * time.Millisecond
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
		lis.Close()
	})
	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	delayv1.RegisterDelayServiceServer(grpcSrv, &podinfogrpc.DelayServer{})
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
		conn.Close()
	})
	healthClient := grpc_health_v1.NewHealthClient(conn)

	httpLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen %v", err)
	}
	httpStarted := make(chan struct{})
	httpSrv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(httpStarted)
		time.Sleep(700 * time.Millisecond)
		_, _ = io.WriteString(w, "ok")
	})}
	go func() {
		_ = httpSrv.Serve(httpLis)
	}()

	delayErr := make(chan error, 1)
	go func() {
		_, err := delayv1.NewDelayServiceClient(conn).Delay(context.Background(), &delayv1.DelayRequest{Seconds: 30})
		delayErr <- err
	}()
	httpResult := make(chan error, 1)
	go func() {
		res, err := http.Get("http://" + httpLis.Addr().String())
		if err == nil {
			res.Body.Close()
			if res.StatusCode != http.StatusOK {
				err = io.ErrUnexpectedEOF
			}
		}
		httpResult <- err
	}()

	select {
	case <-delayStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("Delay call did not reach the server")
	}
	select {
	case <-httpStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP request did not reach the server")
	}

	sd := NewShutdown(shutdownTimeout, drainDelay, zap.NewNop())
	stopCh := make(chan struct{})
	close(stopCh)
	var healthy, ready int32 = 1, 1

	done := make(chan struct{})
	start := time.Now()
	go func() {
		sd.Graceful(stopCh, httpSrv, nil, grpcSrv, healthSrv, &healthy, &ready)
		close(done)
	}()

	drainCheck := start.Add(drainDelay / 2)
	waitNotServing(t, healthClient, "", drainCheck)
	waitNotServing(t, healthClient, "podinfo", drainCheck)
	select {
	case <-done:
		t.Fatal("Graceful returned before the drain delay")
	default:
	}
	if atomic.LoadInt32(&healthy) != 0 || atomic.LoadInt32(&ready) != 0 {
		t.Errorf("healthy=%d ready=%d, want 0", atomic.LoadInt32(&healthy), atomic.LoadInt32(&ready))
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Graceful did not return")
	}
	elapsed := time.Since(start)
	if elapsed < drainDelay+shutdownTimeout {
		t.Errorf("Graceful returned after %s, want >= %s", elapsed, drainDelay+shutdownTimeout)
	}
	if elapsed > drainDelay+shutdownTimeout+time.Second {
		t.Errorf("Graceful returned after %s, want ~%s", elapsed, drainDelay+shutdownTimeout)
	}

	select {
	case err := <-delayErr:
		if status.Code(err) != codes.Unavailable {
			t.Errorf("Delay returned %s, want %s", status.Code(err), codes.Unavailable)
		}
	case <-time.After(5 * time.Second):
		t.Error("in-flight Delay was not interrupted")
	}

	select {
	case err := <-httpResult:
		if err != nil {
			t.Errorf("in-flight HTTP request failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("in-flight HTTP request did not complete")
	}
}

func waitNotServing(t *testing.T, client grpc_health_v1.HealthClient, service string, deadline time.Time) {
	t.Helper()

	for {
		res, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: service})
		if err == nil && res.GetStatus() == grpc_health_v1.HealthCheckResponse_NOT_SERVING {
			return
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("Check(%q) is not NOT_SERVING", service)
}
