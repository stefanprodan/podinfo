// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"

	"go.uber.org/zap"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/env/v1"
	"github.com/stefanprodan/podinfo/pkg/environ"
)

type envServer struct {
	pb.UnimplementedEnvServiceServer
	config *Config
	logger *zap.Logger
}

func (s *envServer) Env(_ context.Context, _ *pb.EnvRequest) (*pb.EnvResponse, error) {
	return &pb.EnvResponse{EnvVars: environ.Redacted()}, nil
}
