package grpc

import (
	"context"
	"errors"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cachev1 "github.com/stefanprodan/podinfo/pkg/api/grpc/cache/v1"
	"github.com/stefanprodan/podinfo/pkg/api/limits"
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
	if len(key) > limits.MaxCacheKeySize {
		return status.Error(codes.InvalidArgument, "key too long")
	}
	// The HTTP API takes the key from one path segment.
	if strings.Contains(key, "/") {
		return status.Error(codes.InvalidArgument, "key must not contain a slash")
	}
	return nil
}

// failed returns the status of a failed cache command: the context status
// when the client canceled or timed out, Internal otherwise.
func (s *cacheServer) failed(ctx context.Context, msg string, err error) error {
	if ctx.Err() != nil {
		return status.FromContextError(ctx.Err()).Err()
	}
	s.logger.Warn(msg, zap.Error(err), telemetry.Context(ctx))
	return status.Error(codes.Internal, msg)
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
		return nil, s.failed(ctx, "cache get failed", err)
	}

	return &cachev1.GetResponse{Value: data}, nil
}

func (s *cacheServer) Set(ctx context.Context, req *cachev1.SetRequest) (*cachev1.SetResponse, error) {
	if err := s.validate(req.GetKey()); err != nil {
		return nil, err
	}
	if err := checkBodySize(len(req.GetValue())); err != nil {
		return nil, err
	}

	if err := s.cache.Set(ctx, req.GetKey(), req.GetValue(), 0).Err(); err != nil {
		return nil, s.failed(ctx, "cache set failed", err)
	}

	return &cachev1.SetResponse{}, nil
}

func (s *cacheServer) Delete(ctx context.Context, req *cachev1.DeleteRequest) (*cachev1.DeleteResponse, error) {
	if err := s.validate(req.GetKey()); err != nil {
		return nil, err
	}

	if err := s.cache.Del(ctx, req.GetKey()).Err(); err != nil {
		return nil, s.failed(ctx, "cache delete failed", err)
	}

	return &cachev1.DeleteResponse{}, nil
}
