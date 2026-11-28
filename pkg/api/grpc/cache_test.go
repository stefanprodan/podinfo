package grpc

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cachev1 "github.com/stefanprodan/podinfo/pkg/api/grpc/cache/v1"
	"github.com/stefanprodan/podinfo/pkg/api/limits"
)

func newCacheClient(t *testing.T, cache redis.UniversalClient) cachev1.CacheServiceClient {
	t.Helper()
	srv := grpc.NewServer()
	cachev1.RegisterCacheServiceServer(srv, &cacheServer{cache: cache, logger: zap.NewNop()})
	return cachev1.NewCacheServiceClient(newBufconnClient(t, srv))
}

func newMiniredisClient(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	t.Helper()
	m := miniredis.RunT(t)
	cache := redis.NewClient(&redis.Options{Addr: m.Addr()})
	t.Cleanup(func() { _ = cache.Close() })
	return m, cache
}

func wantCode(t *testing.T, rpc string, err error, code codes.Code) {
	t.Helper()
	if got := status.Code(err); got != code {
		t.Errorf("%s returned %v (%v), want %v", rpc, got, err, code)
	}
}

func TestGrpcCache(t *testing.T) {
	m, cache := newMiniredisClient(t)
	client := newCacheClient(t, cache)
	ctx := context.Background()
	value := []byte{0xff, 0xfe, 'p', 'o', 'd'}

	if _, err := client.Set(ctx, &cachev1.SetRequest{Key: "key1", Value: value}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got, _ := m.Get("key1"); got != string(value) {
		t.Errorf("cache has %q, want %q", got, value)
	}
	if ttl := m.TTL("key1"); ttl != 0 {
		t.Errorf("cache TTL = %v, want no expiry", ttl)
	}

	res, err := client.Get(ctx, &cachev1.GetRequest{Key: "key1"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(res.GetValue(), value) {
		t.Errorf("Get returned %q, want %q", res.GetValue(), value)
	}

	if _, err := client.Delete(ctx, &cachev1.DeleteRequest{Key: "key1"}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if m.Exists("key1") {
		t.Error("key1 still exists after Delete")
	}

	_, err = client.Get(ctx, &cachev1.GetRequest{Key: "key1"})
	wantCode(t, "Get", err, codes.NotFound)
}

func TestGrpcCache_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("not configured", func(t *testing.T) {
		client := newCacheClient(t, nil)
		_, err := client.Get(ctx, &cachev1.GetRequest{Key: "key1"})
		wantCode(t, "Get", err, codes.Unavailable)
		if msg := status.Convert(err).Message(); msg != "cache server not configured" {
			t.Errorf("Get returned message %q, want %q", msg, "cache server not configured")
		}
		_, err = client.Set(ctx, &cachev1.SetRequest{Key: "key1"})
		wantCode(t, "Set", err, codes.Unavailable)
		_, err = client.Delete(ctx, &cachev1.DeleteRequest{Key: "key1"})
		wantCode(t, "Delete", err, codes.Unavailable)
	})

	t.Run("empty key", func(t *testing.T) {
		_, cache := newMiniredisClient(t)
		client := newCacheClient(t, cache)
		_, err := client.Get(ctx, &cachev1.GetRequest{})
		wantCode(t, "Get", err, codes.InvalidArgument)
		_, err = client.Set(ctx, &cachev1.SetRequest{Value: []byte("value1")})
		wantCode(t, "Set", err, codes.InvalidArgument)
		_, err = client.Delete(ctx, &cachev1.DeleteRequest{})
		wantCode(t, "Delete", err, codes.InvalidArgument)
	})

	t.Run("key too long", func(t *testing.T) {
		m, cache := newMiniredisClient(t)
		client := newCacheClient(t, cache)
		key := strings.Repeat("k", limits.MaxCacheKeySize)
		if _, err := client.Set(ctx, &cachev1.SetRequest{Key: key}); err != nil || !m.Exists(key) {
			t.Errorf("Set with a %d byte key returned %v", len(key), err)
		}
		key += "k"
		_, err := client.Get(ctx, &cachev1.GetRequest{Key: key})
		wantCode(t, "Get", err, codes.InvalidArgument)
		_, err = client.Set(ctx, &cachev1.SetRequest{Key: key})
		wantCode(t, "Set", err, codes.InvalidArgument)
		_, err = client.Delete(ctx, &cachev1.DeleteRequest{Key: key})
		wantCode(t, "Delete", err, codes.InvalidArgument)
	})

	t.Run("key with a slash", func(t *testing.T) {
		m, cache := newMiniredisClient(t)
		client := newCacheClient(t, cache)
		_, err := client.Get(ctx, &cachev1.GetRequest{Key: "users/42"})
		wantCode(t, "Get", err, codes.InvalidArgument)
		_, err = client.Set(ctx, &cachev1.SetRequest{Key: "users/42"})
		wantCode(t, "Set", err, codes.InvalidArgument)
		_, err = client.Delete(ctx, &cachev1.DeleteRequest{Key: "users/42"})
		wantCode(t, "Delete", err, codes.InvalidArgument)
		if m.Exists("users/42") {
			t.Error("Set saved a key with a slash")
		}
	})

	t.Run("value size", func(t *testing.T) {
		_, cache := newMiniredisClient(t)
		s := &Server{config: &Config{}, logger: zap.NewNop()}
		srv := grpc.NewServer(s.serverOptions(newMetrics(prometheus.NewRegistry()))...)
		cachev1.RegisterCacheServiceServer(srv, &cacheServer{cache: cache, logger: s.logger})
		client := cachev1.NewCacheServiceClient(newBufconnClient(t, srv))
		req := &cachev1.SetRequest{Key: strings.Repeat("k", limits.MaxCacheKeySize), Value: make([]byte, limits.MaxBodySize)}
		if _, err := client.Set(ctx, req); err != nil {
			t.Errorf("Set with the longest key and the largest value returned %v", err)
		}
		req.Value = append(req.Value, 0)
		_, err := client.Set(ctx, req)
		wantCode(t, "Set", err, codes.ResourceExhausted)
		if msg := status.Convert(err).Message(); msg != "request body too large" {
			t.Errorf("Set returned message %q, want %q", msg, "request body too large")
		}
	})

	t.Run("client gone", func(t *testing.T) {
		_, cache := newMiniredisClient(t)
		s := &cacheServer{cache: cache, logger: zap.NewNop()}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		_, err := s.Get(canceled, &cachev1.GetRequest{Key: "key1"})
		wantCode(t, "Get", err, codes.Canceled)
		_, err = s.Set(canceled, &cachev1.SetRequest{Key: "key1"})
		wantCode(t, "Set", err, codes.Canceled)
		_, err = s.Delete(canceled, &cachev1.DeleteRequest{Key: "key1"})
		wantCode(t, "Delete", err, codes.Canceled)

		expired, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
		defer cancel()
		_, err = s.Get(expired, &cachev1.GetRequest{Key: "key1"})
		wantCode(t, "Get", err, codes.DeadlineExceeded)
	})

	t.Run("cache error", func(t *testing.T) {
		m, cache := newMiniredisClient(t)
		client := newCacheClient(t, cache)
		m.SetError("LOADING")
		_, err := client.Get(ctx, &cachev1.GetRequest{Key: "key1"})
		wantCode(t, "Get", err, codes.Internal)
		_, err = client.Set(ctx, &cachev1.SetRequest{Key: "key1", Value: []byte("value1")})
		wantCode(t, "Set", err, codes.Internal)
		_, err = client.Delete(ctx, &cachev1.DeleteRequest{Key: "key1"})
		wantCode(t, "Delete", err, codes.Internal)
	})
}
