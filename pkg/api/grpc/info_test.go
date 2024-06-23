package grpc

import (
	"context"
	"log"
	"net"
	"regexp"
	"testing"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/info"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcInfo(t *testing.T) {

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		lis.Close()
	})

	s := NewMockGrpcServer()
	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	info.RegisterInfoServiceServer(srv, &infoServer{config: s.config})

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

	client := info.NewInfoServiceClient(conn)
	res, err := client.Info(context.Background(), &info.InfoRequest{})

	if _, ok := status.FromError(err); !ok {
		t.Errorf("Info returned type %T, want %T", err, status.Error)
	}

	expected := ".*color.*blue.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(res.String()) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res.Color, expected)
	}
}
