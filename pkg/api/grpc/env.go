package grpc

import (
	"context"

	"go.uber.org/zap"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/env/v1"
	"github.com/stefanprodan/podinfo/pkg/environ"
)

type EnvServer struct {
	pb.UnimplementedEnvServiceServer
	config *Config
	logger *zap.Logger
}

func (s *EnvServer) Env(ctx context.Context, envInput *pb.EnvRequest) (*pb.EnvResponse, error) {
	return &pb.EnvResponse{EnvVars: environ.Redacted()}, nil
}
