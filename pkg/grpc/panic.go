package grpc

import (
	"context"
	"os"
	"go.uber.org/zap"
	pb "github.com/stefanprodan/podinfo/pkg/grpc/panic"
)

type PanicServer struct {
	pb.UnimplementedPanicServiceServer
	logger *zap.Logger
}

// SayHello implements helloworld.GreeterServer

func (s *PanicServer) Panic(ctx context.Context, req *pb.PanicRequest) (*pb.PanicResponse, error) {
	s.logger.Info("Panic command received")
	os.Exit(225)
	return &pb.PanicResponse{}, nil
}

