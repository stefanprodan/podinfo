package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"testing"

	versionv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/version/v1"
	v "github.com/stefanprodan/podinfo/pkg/version"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcVersion(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		_ = lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	versionv1.RegisterVersionServiceServer(srv, &versionServer{})

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

	client := versionv1.NewVersionServiceClient(conn)
	res, err := client.Version(context.Background(), &versionv1.VersionRequest{})

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
