package grpc

import (
	"context"
	"log"
	"os"

	pb "github.com/stefanprodan/podinfo/pkg/grpc/panic"
	"go.uber.org/zap"
)

type PanicServer struct {
	pb.UnimplementedPanicServiceServer
	logger *zap.Logger
}

// SayHello implements helloworld.GreeterServer

func (s *PanicServer) Panic(ctx context.Context, req *pb.PanicRequest) (*pb.PanicResponse, error) {
	
	if(s.logger == nil) {log.Printf("S.log is nil")}
	s.logger.Info("Panic command received")
	os.Exit(225)
	return &pb.PanicResponse{}, nil
}

