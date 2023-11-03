package grpc

import (
	"context"
	"strings"

	"github.com/stefanprodan/podinfo/pkg/grpc/header"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type headerServer struct {
	header.UnimplementedHeaderServiceServer
}

func (s *headerServer) Header (ctx context.Context, in *header.HeaderRequest) (*header.HeaderResponse, error) {

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.DataLoss, "UnaryEcho: failed to get metadata")
	}

	// Creating slices beacause echoing the header metadata can't be predetermined by the proto contract
	res := []string{}
	for i,e := range md {
		res = append(res, i + "=" + strings.Join(e, ","))
	}

	return &header.HeaderResponse{Headers: res}, nil

}
