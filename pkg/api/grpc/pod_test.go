package grpc

import (
	"context"
	"net"
	"slices"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/pod"
)

func TestGrpcPod(t *testing.T) {
	t.Setenv("PODINFO_TEST_PLAIN", "plain")
	t.Setenv("PODINFO_TEST_PASSWORD", "s3cret")

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { lis.Close() })

	srv := grpc.NewServer()
	t.Cleanup(srv.Stop)
	pod.RegisterPodServiceServer(srv, &podServer{config: NewMockGrpcServer().config})
	go func() { _ = srv.Serve(lis) }()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-test", "podinfo", "x-spiffe-id", "spiffe://example.org/forged", "authorization", "Bearer s3cret")
	res, err := pod.NewPodServiceClient(conn).Pod(ctx, &pod.PodRequest{})
	if err != nil {
		t.Fatalf("Pod: %v", err)
	}

	if res.Hostname != "localhost" {
		t.Errorf("hostname = %q, want %q", res.Hostname, "localhost")
	}
	if res.Runtime == "" || res.NumCpu == "" {
		t.Errorf("runtime fields missing: %v", res)
	}
	if got := res.Env["PODINFO_TEST_PLAIN"]; got != "plain" {
		t.Errorf("env PODINFO_TEST_PLAIN = %q, want %q", got, "plain")
	}
	if got := res.Env["PODINFO_TEST_PASSWORD"]; got != "[REDACTED]" {
		t.Errorf("env PODINFO_TEST_PASSWORD = %q, want it masked", got)
	}
	if got := res.Headers["x-test"].GetValues(); !slices.Equal(got, []string{"podinfo"}) {
		t.Errorf("header x-test = %v, want [podinfo]", got)
	}
	if got := res.Headers["authorization"].GetValues(); !slices.Equal(got, []string{"[REDACTED]"}) {
		t.Errorf("header authorization = %v, want it masked", got)
	}
	for name := range res.Headers {
		if strings.HasPrefix(name, ":") {
			t.Errorf("pseudo-header %s echoed", name)
		}
	}
	if got, ok := res.Headers["x-spiffe-id"]; ok {
		t.Errorf("client-supplied x-spiffe-id echoed: %v", got)
	}
}
