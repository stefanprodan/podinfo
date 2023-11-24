package grpc

import (
	"context"
	"log"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/echo"
	"go.uber.org/zap"
)

type echoServer struct {
	echo.UnimplementedEchoServiceServer
	config *Config
	logger *zap.Logger
}

func (s *echoServer) Echo (ctx context.Context, message *echo.Message) (*echo.Message, error){
	log.Printf("Received message body from client: %s", message.Body)
	return &echo.Message {Body: message.Body}, nil
}
