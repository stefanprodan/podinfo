package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"testing"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	st "google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcStatusError(t *testing.T) {

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	status.RegisterStatusServiceServer(srv, &StatusServer{})

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

	client := status.NewStatusServiceClient(conn)

	res, err := client.Status(context.Background(), &status.StatusRequest{Code: "NotFound"})

	if err != nil {
		if e, ok := st.FromError(err); ok {
			if e.Code() != codes.NotFound {
				if res != nil {
					fmt.Printf("res %v\n", res)
				}
				t.Errorf("Status returned %s, want %s", fmt.Sprint(e.Code()), fmt.Sprint(codes.Aborted))
			}
		}
	}

}

func TestGrpcStatusOk(t *testing.T) {

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	status.RegisterStatusServiceServer(srv, &StatusServer{})

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

	client := status.NewStatusServiceClient(conn)

	res, err := client.Status(context.Background(), &status.StatusRequest{Code: "Ok"})
	if err != nil {
		t.Fatalf("Status returned %s, want %s", fmt.Sprint(st.Code(err)), fmt.Sprint(codes.OK))
	}

	expected := ".*Ok.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(res.Status) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res, expected)
	}

}
