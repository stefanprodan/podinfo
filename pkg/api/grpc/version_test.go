package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"testing"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/version"
	v "github.com/stefanprodan/podinfo/pkg/version"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcVersion(t *testing.T) {

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	version.RegisterVersionServiceServer(srv, &VersionServer{})

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

	client := version.NewVersionServiceClient(conn)
	res, err := client.Version(context.Background(), &version.VersionRequest{})

	if _, ok := status.FromError(err); !ok {
		t.Errorf("Version returned type %T, want %T", err, status.Error)
	}

	expected := fmt.Sprintf(".*%s.*", v.VERSION)
	r := regexp.MustCompile(expected)
	if !r.MatchString(res.String()) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res, expected)
	}
}
