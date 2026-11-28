// Copyright 2023 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/version/v1"
	"github.com/stefanprodan/podinfo/pkg/version"
	"go.uber.org/zap"
)

type versionServer struct {
	pb.UnimplementedVersionServiceServer
	config *Config
	logger *zap.Logger
}

func (s *versionServer) Version(_ context.Context, _ *pb.VersionRequest) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{Version: version.VERSION, Commit: version.REVISION}, nil
}
