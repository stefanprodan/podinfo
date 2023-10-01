package grpc

import (
	"context"

	pb "github.com/stefanprodan/podinfo/pkg/grpc/version"
	"github.com/stefanprodan/podinfo/pkg/version"
)

type VersionServer struct {
	pb.UnimplementedVersionServiceServer
}

// SayHello implements helloworld.GreeterServer

func (s *VersionServer) Version(ctx context.Context, req *pb.VersionRequest) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{Version: version.VERSION, Commit: version.REVISION}, nil
}

// var VersionService := &version{
// 	// ...
// }
