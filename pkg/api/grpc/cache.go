package grpc

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cachev1 "github.com/stefanprodan/podinfo/pkg/api/grpc/cache/v1"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

type cacheServer struct {
	cachev1.UnimplementedCacheServiceServer
	cache  redis.UniversalClient
	logger *zap.Logger
}

var errCacheOffline = status.Error(codes.Unavailable, "cache server is offline")

func (s *cacheServer) validate(key string) error {
	if s.cache == nil {
		return errCacheOffline
	}
	if key == "" {
		return status.Error(codes.InvalidArgument, "key is required")
	}
	return nil
}

func (s *cacheServer) Get(ctx context.Context, req *cachev1.GetRequest) (*cachev1.GetResponse, error) {
	if err := s.validate(req.GetKey()); err != nil {
		return nil, err
	}

	data, err := s.cache.Get(ctx, req.GetKey()).Bytes()
	if errors.Is(err, redis.Nil) {
		s.logger.Warn("cache key not found", zap.String("key", req.GetKey()), telemetry.Context(ctx))
		return nil, status.Error(codes.NotFound, "key not found")
	}
	if err != nil {
		s.logger.Warn("cache get failed", zap.Error(err), telemetry.Context(ctx))
		return nil, status.Error(codes.Internal, "cache get failed")
	}

	return &cachev1.GetResponse{Value: data}, nil
}

func (s *cacheServer) Set(ctx context.Context, req *cachev1.SetRequest) (*cachev1.SetResponse, error) {
	if err := s.validate(req.GetKey()); err != nil {
		return nil, err
	}

	if err := s.cache.Set(ctx, req.GetKey(), req.GetValue(), 0).Err(); err != nil {
		s.logger.Warn("cache set failed", zap.Error(err), telemetry.Context(ctx))
		return nil, status.Error(codes.Internal, "cache set failed")
	}

	return &cachev1.SetResponse{}, nil
}

func (s *cacheServer) Delete(ctx context.Context, req *cachev1.DeleteRequest) (*cachev1.DeleteResponse, error) {
	if err := s.validate(req.GetKey()); err != nil {
		return nil, err
	}

	if err := s.cache.Del(ctx, req.GetKey()).Err(); err != nil {
		s.logger.Warn("cache delete failed", zap.Error(err), telemetry.Context(ctx))
		return nil, status.Error(codes.Internal, "cache delete failed")
	}

	return &cachev1.DeleteResponse{}, nil
}
