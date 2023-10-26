package grpc

import (
	"context"
	"os"

	"github.com/stefanprodan/podinfo/pkg/grpc/env"
)

type envServer struct {
	env.UnimplementedEnvServiceServer
}

func (s *envServer) Env (ctx context.Context, envInput *env.EnvRequest) (*env.EnvResponse, error){
	return &env.EnvResponse{EnvVars: os.Environ()}, nil
}