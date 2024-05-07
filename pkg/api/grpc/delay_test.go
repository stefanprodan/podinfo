package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"testing"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/delay"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcDelay(t *testing.T) {

	// Server initialization
	// bufconn => uses in-memory connection instead of system network I/O
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	delay.RegisterDelayServiceServer(srv, &DelayServer{})

	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("srv.Serve %v", err)
		}
	}()

	// - Test
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

	client := delay.NewDelayServiceClient(conn)
	res, err := client.Delay(context.Background(), &delay.DelayRequest{Seconds: 3})

	// Check the status code is what we expect.
	if _, ok := status.FromError(err); !ok {
		t.Errorf("Delay returned type %T, want %T", err, status.Error)
	}

	if res != nil {
		fmt.Printf("res %v\n", res)
	}

	// Check the response body is what we expect. Here we expect the response to be "3" as the delay is set to 3 seconds.
	expected := "3"
	r := regexp.MustCompile(expected)
	if !r.MatchString(strconv.FormatInt(res.Message, 10)) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res.Message, expected)
	}
}
