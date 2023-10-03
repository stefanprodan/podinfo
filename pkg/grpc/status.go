package grpc

import (
	"context"

	pb "github.com/stefanprodan/podinfo/pkg/grpc/status"
)

type StatusServer struct {
	pb.UnimplementedStatusServiceServer
}

// SayHello implements helloworld.GreeterServer

func (s *StatusServer) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	reqCode := req.GetCode()
	return &pb.StatusResponse{Status: reqCode}, nil
}
