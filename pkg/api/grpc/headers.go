package grpc

import (
	"context"
	"strings"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/header/v1"
	"github.com/stefanprodan/podinfo/pkg/environ"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type HeaderServer struct {
	pb.UnimplementedHeaderServiceServer
	config *Config
	logger *zap.Logger
}

func (s *HeaderServer) Header(ctx context.Context, in *pb.HeaderRequest) (*pb.HeaderResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.DataLoss, "failed to get metadata")
	}

	// Creating slices beacause echoing the header metadata can't be predetermined by the proto contract
	res := []string{}
	for i, e := range environ.RedactHeaders(md) {
		res = append(res, i+"="+strings.Join(e, ","))
	}

	return &pb.HeaderResponse{Headers: res}, nil

}
