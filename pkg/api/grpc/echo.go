package grpc

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"

	echov1 "github.com/stefanprodan/podinfo/pkg/api/grpc/echo/v1"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
	"github.com/stefanprodan/podinfo/pkg/version"
)

type echoServer struct {
	echov1.UnimplementedEchoServiceServer
	config   *Config
	logger   *zap.Logger
	backends []echoBackend
}

// echoBackend is the Echo client of one --grpc-backend-url.
type echoBackend struct {
	url    string
	client echov1.EchoServiceClient
}

// Echo returns the body and calls Echo on every backend concurrently,
// reporting their responses in --grpc-backend-url order.
func (s *echoServer) Echo(ctx context.Context, req *echov1.EchoRequest) (*echov1.EchoResponse, error) {
	s.logger.Debug("received message body from client", zap.String("body", req.Body), telemetry.Context(ctx))
	res := &echov1.EchoResponse{Body: req.Body}
	if len(s.backends) == 0 {
		return res, nil
	}

	md := metadata.Pairs("x-api-version", version.VERSION, "x-api-revision", version.REVISION)
	if ids := metadata.ValueFromIncomingContext(ctx, "x-request-id"); len(ids) > 0 {
		md.Set("x-request-id", ids[0])
	}
	callCtx := metadata.NewOutgoingContext(ctx, md)
	if s.config.ClientTimeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(callCtx, s.config.ClientTimeout)
		defer cancel()
	}

	span := trace.SpanFromContext(ctx)
	res.BackendResponses = make([]*echov1.BackendResponse, len(s.backends))
	var wg sync.WaitGroup
	for i, b := range s.backends {
		br := &echov1.BackendResponse{Url: b.url}
		res.BackendResponses[i] = br
		wg.Go(func() {
			r, err := b.client.Echo(callCtx, &echov1.EchoRequest{Body: req.Body})
			if err != nil {
				s.logger.Error("backend call failed", zap.Error(err), zap.String("url", b.url), telemetry.Context(ctx))
				span.RecordError(err)
				br.Error = err.Error()
				return
			}
			s.logger.Debug(
				"payload received from backend",
				zap.String("response", r.GetBody()),
				zap.String("url", b.url),
				telemetry.Context(ctx))
			br.Body = r.GetBody()
		})
	}
	wg.Wait()
	return res, nil
}
