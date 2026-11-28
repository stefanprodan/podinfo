// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"
	"log"
	"net"
	"regexp"
	"testing"

	infov1 "github.com/stefanprodan/podinfo/pkg/api/grpc/info/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcInfo(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		_ = lis.Close()
	})

	s := NewMockGrpcServer()
	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	infov1.RegisterInfoServiceServer(srv, &infoServer{config: s.config})

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

	client := infov1.NewInfoServiceClient(conn)
	res, err := client.Info(context.Background(), &infov1.InfoRequest{})

	if _, ok := status.FromError(err); !ok {
		t.Errorf("Info returned type %T, want %T", err, status.Error)
	}

	expected := ".*color.*blue.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(res.String()) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res.GetColor(), expected)
	}
}
