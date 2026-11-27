package grpc

import (
	"context"

	echov1 "github.com/stefanprodan/podinfo/pkg/api/grpc/echo/v1"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
	"go.uber.org/zap"
)

type echoServer struct {
	echov1.UnimplementedEchoServiceServer
	config *Config
	logger *zap.Logger
}

func (s *echoServer) Echo(ctx context.Context, req *echov1.EchoRequest) (*echov1.EchoResponse, error) {
	s.logger.Debug("received message body from client", zap.String("body", req.Body), telemetry.Context(ctx))
	return &echov1.EchoResponse{Body: req.Body}, nil
}
