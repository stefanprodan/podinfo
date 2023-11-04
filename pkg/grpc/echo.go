package grpc

import (
	"context"
	"log"

	"github.com/stefanprodan/podinfo/pkg/grpc/echo"
)

type echoServer struct {
	 echo.UnimplementedEchoServiceServer
}

func (s *echoServer) Echo (ctx context.Context, message *echo.Message) (*echo.Message, error){
	log.Printf("Received message body from client: %s", message.Body)
	return &echo.Message {Body: message.Body}, nil
}
