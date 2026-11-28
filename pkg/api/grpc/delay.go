package grpc

import (
	"context"
	"time"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/delay/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"
)

type delayServer struct {
	pb.UnimplementedDelayServiceServer
	config *Config
	logger *zap.Logger
}

func (s *delayServer) Delay(ctx context.Context, req *pb.DelayRequest) (*pb.DelayResponse, error) {
	timer := time.NewTimer(time.Duration(req.GetSeconds()) * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
		return &pb.DelayResponse{Message: req.GetSeconds()}, nil
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
}
