package grpc

import (
	"context"
	"time"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/delay"
)

type delayServer struct {
	delay.UnimplementedDelayServiceServer
}

func (s *delayServer) Delay (ctx context.Context, delayInput *delay.DelayRequest) (*delay.Response, error){

	// if &delayInput.Seconds == nil {


		
	// }

	time.Sleep(time.Duration(delayInput.Seconds) * time.Second)
	return &delay.Response {Message: delayInput.Seconds},nil
}