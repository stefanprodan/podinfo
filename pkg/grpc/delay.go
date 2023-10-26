package grpc

import (
	"context"
	"time"

	"github.com/stefanprodan/podinfo/pkg/grpc/delay"
)

type delayServer struct {
	delay.UnimplementedDelayServiceServer
}

func (s *delayServer) Delay (ctx context.Context, delayInput *delay.DelayRequest) (*delay.Response, error){
	time.Sleep(time.Duration(delayInput.Seconds) * time.Second)
	return &delay.Response {Message: delayInput.Seconds},nil
}