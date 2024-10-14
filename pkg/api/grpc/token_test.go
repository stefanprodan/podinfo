package grpc

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/token"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcToken(t *testing.T) {

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})
	config := &Config{}
	config.JWTSecret = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	token.RegisterTokenServiceServer(srv, &TokenServer{config: config})

	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("srv.Serve %v", err)
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	ctx := context.Background()

	conn, err := grpc.DialContext(ctx, "", grpc.WithContextDialer(dialer), grpc.WithInsecure())
	t.Cleanup(func() {
		conn.Close()
	})

	if err != nil {
		t.Fatalf("grpc.DialContext %v", err)
	}

	client := token.NewTokenServiceClient(conn)
	res, err := client.TokenGenerate(context.Background(), &token.TokenRequest{})

	if _, ok := status.FromError(err); !ok {
		t.Errorf("Token Handler returned type %T, want %T", err, status.Error)
	}

	var token = token.TokenResponse{
		Token:     res.Token,
		ExpiresAt: res.ExpiresAt,
	}

	if token.Token == "" {
		t.Fatalf("Handler returned no token")
	}
}
