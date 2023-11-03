package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"testing"

	"github.com/stefanprodan/podinfo/pkg/grpc/info"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestInfo(t *testing.T) {

	// Server initialization
	// bufconn => uses in-memory connection instead of system network I/O
	lis := bufconn.Listen(1024*1024)
	t.Cleanup(func() {
		lis.Close()
	})

	s := NewMockGrpcServer()
	srv := grpc.NewServer() // replace this with Mock that return srv that has all the config, logger, etc
	t.Cleanup(func() {
		srv.Stop()
	})

	//srv := info.infoServer{}
	info.RegisterInfoServiceServer(srv, &infoServer{config: s.config})

	go func(){
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("srv.Serve %v", err)
		}
	}()

	// - Test
	dialer := func(context.Context, string) (net.Conn, error){
		return lis.Dial()
	}

	ctx := context.Background()
	
	// conn , err := grpc.DialContext(context.Background(), "", grpc.WithTransportCredentials(insecure.NewCredentials()),grpc.FailOnNonTempDialError(true))
	conn, err := grpc.DialContext(ctx, "", grpc.WithContextDialer(dialer), grpc.WithInsecure())
	t.Cleanup(func() {
		conn.Close()
	})

	if err != nil {
		t.Fatalf("grpc.DialContext %v", err)
	}

	client := info.NewInfoServiceClient(conn)
	res, err := client.Info(context.Background(), &info.InfoRequest{})

	// Check the status code is what we expect.
	if _, ok := status.FromError(err); !ok {
		t.Errorf("Info returned type %T, want %T", err, status.Error)
	}

	if res != nil {
		fmt.Printf("res %v\n", res)
		// fmt.Printf(res.Color, " ", reflect.TypeOf(res.Color))
	}

	// Check the response body is what we expect.
	expected := ".*color.*blue.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(res.Color) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res.Color, expected)
	}
}