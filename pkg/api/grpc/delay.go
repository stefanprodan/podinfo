package grpc

import (
	"context"
	"time"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/delay"
	"go.uber.org/zap"
)

type DelayServer struct {
	pb.UnimplementedDelayServiceServer
	config *Config
	logger *zap.Logger
}

func (s *DelayServer) Delay(ctx context.Context, delayInput *pb.DelayRequest) (*pb.DelayResponse, error) {

	time.Sleep(time.Duration(delayInput.Seconds) * time.Second)
	return &pb.DelayResponse{Message: delayInput.Seconds}, nil
}
