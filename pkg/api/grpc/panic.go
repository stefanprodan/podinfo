package grpc

import (
	"context"
	"os"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/panic"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
	"go.uber.org/zap"
)

type PanicServer struct {
	pb.UnimplementedPanicServiceServer
	config *Config
	logger *zap.Logger
}

func (s *PanicServer) Panic(ctx context.Context, req *pb.PanicRequest) (*pb.PanicResponse, error) {
	s.logger.Info("Panic command received", telemetry.Context(ctx))
	os.Exit(225)
	return &pb.PanicResponse{}, nil
}
