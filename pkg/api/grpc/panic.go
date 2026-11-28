// Copyright 2023 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"
	"os"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/panic/v1"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
	"go.uber.org/zap"
)

type panicServer struct {
	pb.UnimplementedPanicServiceServer
	config *Config
	logger *zap.Logger
}

func (s *panicServer) Panic(ctx context.Context, _ *pb.PanicRequest) (*pb.PanicResponse, error) {
	s.logger.Info("Panic command received", telemetry.Context(ctx))
	os.Exit(225)
	return &pb.PanicResponse{}, nil
}
