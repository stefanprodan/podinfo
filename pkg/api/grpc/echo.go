package grpc

import (
	"context"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/echo"
	"go.uber.org/zap"
)

type echoServer struct {
	echo.UnimplementedEchoServiceServer
	config *Config
	logger *zap.Logger
}

func (s *echoServer) Echo (ctx context.Context, message *echo.Message) (*echo.Message, error){
	// Log level 0 for Info
	s.logger.Log(0,"Received message body from client:", zap.String("input body", message.Body))
	return &echo.Message {Body: message.Body}, nil
}
