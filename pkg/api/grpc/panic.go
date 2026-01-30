package grpc

import (
	"context"
	// "log"
	"os"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/panic"
	"go.uber.org/zap"
)

type PanicServer struct {
	pb.UnimplementedPanicServiceServer
	config *Config
	logger *zap.Logger
}

func (s *PanicServer) Panic(ctx context.Context, req *pb.PanicRequest) (*pb.PanicResponse, error) {
	s.logger.Info("Panic command received")
	os.Exit(225)
	return &pb.PanicResponse{}, nil
}
