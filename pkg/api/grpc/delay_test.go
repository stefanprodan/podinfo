// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"testing"
	"time"

	delayv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/delay/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcDelay(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		_ = lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})

	delayv1.RegisterDelayServiceServer(srv, &delayServer{})

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

	client := delayv1.NewDelayServiceClient(conn)
	res, err := client.Delay(context.Background(), &delayv1.DelayRequest{Seconds: 1})

	if _, ok := status.FromError(err); !ok {
		t.Errorf("Delay returned type %T, want %T", err, status.Error)
	}

	if res != nil {
		fmt.Printf("res %v\n", res)
	}

	expected := "1"
	r := regexp.MustCompile(expected)
	if !r.MatchString(strconv.FormatInt(res.GetMessage(), 10)) {
		t.Fatalf("Returned unexpected body:\ngot \n%v \nwant \n%s",
			res.GetMessage(), expected)
	}
}

func TestGrpcDelayContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
		want codes.Code
	}{
		{
			name: "deadline exceeded",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 100*time.Millisecond)
			},
			want: codes.DeadlineExceeded,
		},
		{
			name: "canceled",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				time.AfterFunc(100*time.Millisecond, cancel)
				return ctx, cancel
			},
			want: codes.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.ctx()
			defer cancel()

			start := time.Now()
			_, err := (&delayServer{}).Delay(ctx, &delayv1.DelayRequest{Seconds: 3600})
			if elapsed := time.Since(start); elapsed > waitTimeout {
				t.Fatalf("Delay returned after %s", elapsed)
			}
			if code := status.Code(err); code != tt.want {
				t.Errorf("Delay returned %s, want %s", code, tt.want)
			}
		})
	}
}
