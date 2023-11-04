package grpc

import (
	"context"
	"log"
	"net"
	"regexp"
	"testing"

	"github.com/stefanprodan/podinfo/pkg/grpc/header"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcHeader(t *testing.T) {

	// Server initialization
	// bufconn => uses in-memory connection instead of system network I/O
	lis := bufconn.Listen(1024*1024)
	t.Cleanup(func() {
		lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	header.RegisterHeaderServiceServer(srv, &headerServer{})

	go func(){
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("srv.Serve %v", err)
		}
	}()

	// - Test
	dialer := func(context.Context, string) (net.Conn, error){
		return lis.Dial()
	}
	
	conn, err := grpc.DialContext(context.Background(), "", grpc.WithContextDialer(dialer), grpc.WithInsecure())
	t.Cleanup(func() {
		conn.Close()
	})

	if err != nil {
		t.Fatalf("grpc.DialContext %v", err)
	}

	// Prepare your headers as key-value pairs.
	headers := metadata.New(map[string]string{
		"X-Test": "testing",
	})

	// Create a context with the headers attached.
	ctx := metadata.NewOutgoingContext(context.Background(), headers)

	client := header.NewHeaderServiceClient(conn)
	res , err := client.Header(ctx, &header.HeaderRequest{})

	// Check the status code is what we expect.
	if _, ok := status.FromError(err); !ok {
		t.Errorf("Header returned type %T, want %T", err, status.Error)
	}
	// if res != nil {
	// 	fmt.Printf("res %v\n", res)
	// 	// fmt.Printf(res.Color, " ", reflect.TypeOf(res.Color))
	// }

	// Check the response body is what we expect.
	expected := ".*testing.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(res.String()) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res, expected)
	}
}