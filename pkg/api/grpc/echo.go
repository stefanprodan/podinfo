package grpc

import (
	"context"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/echo"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
	"go.uber.org/zap"
)

type echoServer struct {
	echo.UnimplementedEchoServiceServer
	config *Config
	logger *zap.Logger
}

func (s *echoServer) Echo(ctx context.Context, message *echo.Message) (*echo.Message, error) {
	s.logger.Debug("received message body from client", zap.String("body", message.Body), telemetry.Context(ctx))
	return &echo.Message{Body: message.Body}, nil
}
