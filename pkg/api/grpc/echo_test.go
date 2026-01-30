package grpc

import (
	"context"
	"log"
	"net"
	"regexp"
	"testing"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/echo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcEcho(t *testing.T) {

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		lis.Close()
	})

	s := NewMockGrpcServer()
	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	echo.RegisterEchoServiceServer(srv, &echoServer{config: s.config, logger: s.logger})

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

	client := echo.NewEchoServiceClient(conn)
	res, err := client.Echo(context.Background(), &echo.Message{Body: "test123-test"})

	if _, ok := status.FromError(err); !ok {
		t.Errorf("Echo returned type %T, want %T", err, status.Error)
	}

	expected := ".*body.*test123-test.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(res.String()) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res, expected)
	}
}
