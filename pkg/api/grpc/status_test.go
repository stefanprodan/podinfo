package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"testing"

	statusv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/status/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	st "google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcStatusError(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		_ = lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	statusv1.RegisterStatusServiceServer(srv, &statusServer{})

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

	client := statusv1.NewStatusServiceClient(conn)

	res, err := client.Status(context.Background(), &statusv1.StatusRequest{Code: "NotFound"})

	if err != nil {
		if e, ok := st.FromError(err); ok {
			if e.Code() != codes.NotFound {
				if res != nil {
					fmt.Printf("res %v\n", res)
				}
				t.Errorf("Status returned %s, want %s", e.Code().String(), codes.Aborted.String())
			}
		}
	}
}

func TestGrpcStatusOk(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		_ = lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	statusv1.RegisterStatusServiceServer(srv, &statusServer{})

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

	client := statusv1.NewStatusServiceClient(conn)

	res, err := client.Status(context.Background(), &statusv1.StatusRequest{Code: "Ok"})
	if err != nil {
		t.Fatalf("Status returned %s, want %s", st.Code(err).String(), codes.OK.String())
	}

	expected := ".*Ok.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(res.GetStatus()) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res, expected)
	}
}
