package grpc

import (
	"context"
	"log"
	"net"
	"regexp"
	"slices"
	"strings"
	"testing"

	envv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/env/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcEnv(t *testing.T) {
	t.Setenv("PODINFO_CACHE_PASSWORD", "s3cret-value")

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		_ = lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	envv1.RegisterEnvServiceServer(srv, &envServer{})

	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("srv.Serve %v", err)
		}
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

	client := envv1.NewEnvServiceClient(conn)
	res, err := client.Env(context.Background(), &envv1.EnvRequest{})

	if _, ok := status.FromError(err); !ok {
		t.Errorf("Env returned type %T, want %T", err, status.Error)
	}

	expected := ".*PATH.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(res.String()) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res, expected)
	}

	if !slices.Contains(res.GetEnvVars(), "PODINFO_CACHE_PASSWORD=[REDACTED]") {
		t.Errorf("EnvVars does not contain redacted PODINFO_CACHE_PASSWORD")
	}
	if strings.Contains(res.String(), "s3cret-value") {
		t.Error("EnvVars leaks PODINFO_CACHE_PASSWORD value")
	}
}
