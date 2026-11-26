package grpc

import (
	"context"
	"log"
	"net"
	"regexp"
	"testing"

	header "github.com/stefanprodan/podinfo/pkg/api/grpc/headers"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcHeader(t *testing.T) {

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	header.RegisterHeaderServiceServer(srv, &HeaderServer{})

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
		conn.Close()
	})

	headers := metadata.New(map[string]string{
		"X-Test": "testing",
	})

	ctx := metadata.NewOutgoingContext(context.Background(), headers)

	client := header.NewHeaderServiceClient(conn)
	res, err := client.Header(ctx, &header.HeaderRequest{})

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
