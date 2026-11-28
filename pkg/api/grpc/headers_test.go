package grpc

import (
	"context"
	"log"
	"net"
	"regexp"
	"slices"
	"strings"
	"testing"

	headerv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/header/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcHeader(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		_ = lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	headerv1.RegisterHeaderServiceServer(srv, &headerServer{})

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

	headers := metadata.New(map[string]string{
		"X-Test": "testing",
	})

	ctx := metadata.NewOutgoingContext(context.Background(), headers)

	client := headerv1.NewHeaderServiceClient(conn)
	res, err := client.Header(ctx, &headerv1.HeaderRequest{})

	if _, ok := status.FromError(err); !ok {
		t.Errorf("Header returned type %T, want %T", err, status.Error)
	}

	expected := ".*testing.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(res.String()) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res, expected)
	}
}

func TestGrpcHeader_ForgedSPIFFEID(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })

	srv := grpc.NewServer()
	t.Cleanup(srv.Stop)
	headerv1.RegisterHeaderServiceServer(srv, &headerServer{})
	go func() { _ = srv.Serve(lis) }()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-test", "podinfo", "x-spiffe-id", "spiffe://example.org/forged")
	res, err := headerv1.NewHeaderServiceClient(conn).Header(ctx, &headerv1.HeaderRequest{})
	if err != nil {
		t.Fatalf("Header: %v", err)
	}

	if !slices.Contains(res.GetHeaders(), "x-test=podinfo") {
		t.Errorf("headers = %v, want x-test=podinfo", res.GetHeaders())
	}
	for _, h := range res.GetHeaders() {
		if strings.HasPrefix(h, "x-spiffe-id=") {
			t.Errorf("client-supplied x-spiffe-id echoed: %s", h)
		}
	}
}
