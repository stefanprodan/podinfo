package grpc

import (
	"context"
	"errors"
	"io/fs"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	storev1 "github.com/stefanprodan/podinfo/pkg/api/grpc/store/v1"
	"github.com/stefanprodan/podinfo/pkg/store"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

type storeServer struct {
	storev1.UnimplementedStoreServiceServer
	store  *store.Store
	logger *zap.Logger
}

func (s *storeServer) Write(ctx context.Context, req *storev1.WriteRequest) (*storev1.WriteResponse, error) {
	hash, err := s.store.Write(ctx, req.GetContent())
	if err != nil {
		s.logger.Warn("writing file failed", zap.Error(err), telemetry.Context(ctx))
		return nil, status.Error(codes.Internal, "writing file failed")
	}
	return &storev1.WriteResponse{Hash: hash}, nil
}

func (s *storeServer) Read(ctx context.Context, req *storev1.ReadRequest) (*storev1.ReadResponse, error) {
	content, err := s.store.Read(ctx, req.GetHash())
	if errors.Is(err, store.ErrInvalidHash) {
		return nil, status.Error(codes.InvalidArgument, "invalid hash")
	}
	if err != nil {
		s.logger.Warn("reading file failed", zap.Error(err), telemetry.Context(ctx))
		if errors.Is(err, fs.ErrNotExist) {
			return nil, status.Error(codes.NotFound, "file not found")
		}
		return nil, status.Error(codes.Internal, "reading file failed")
	}
	return &storev1.ReadResponse{Content: content}, nil
}
