package grpc

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"

	cachev1 "github.com/stefanprodan/podinfo/pkg/api/grpc/cache/v1"
)

func TestGrpcServerServices(t *testing.T) {
	s := &Server{logger: zap.NewNop(), config: &Config{}}
	srv, _ := s.newServer()
	for _, name := range []string{"cache.v1.CacheService", "store.v1.StoreService"} {
		if _, ok := srv.GetServiceInfo()[name]; !ok {
			t.Errorf("%s is not registered", name)
		}
	}

	_, err := cachev1.NewCacheServiceClient(newBufconnClient(t, srv)).Get(context.Background(), &cachev1.GetRequest{Key: "key1"})
	wantCode(t, "Cache Get without a cache", err, codes.Unavailable)
}
