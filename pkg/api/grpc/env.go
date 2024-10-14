package grpc

import (
	"context"
	"go.uber.org/zap"
	"os"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/env"
)

type EnvServer struct {
	pb.UnimplementedEnvServiceServer
	config *Config
	logger *zap.Logger
}

func (s *EnvServer) Env(ctx context.Context, envInput *pb.EnvRequest) (*pb.EnvResponse, error) {
	return &pb.EnvResponse{EnvVars: os.Environ()}, nil
}
