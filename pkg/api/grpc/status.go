package grpc

import (
	"context"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/status"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type StatusServer struct {
	pb.UnimplementedStatusServiceServer
	config *Config
	logger *zap.Logger
}

func (s *StatusServer) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	reqCode := req.GetCode()

	grpcCodes := map[string]codes.Code{
		"Ok":                 codes.OK,
		"Canceled":           codes.Canceled,
		"Unknown":            codes.Unknown,
		"InvalidArgument":    codes.InvalidArgument,
		"DeadlineExceeded":   codes.DeadlineExceeded,
		"NotFound":           codes.NotFound,
		"AlreadyExists":      codes.AlreadyExists,
		"PermissionDenied":   codes.PermissionDenied,
		"ResourceExhausted":  codes.ResourceExhausted,
		"FailedPrecondition": codes.FailedPrecondition,
		"Aborted":            codes.Aborted,
		"OutOfRange":         codes.OutOfRange,
		"Unimplemented":      codes.Unimplemented,
		"Internal":           codes.Internal,
		"Unavailable":        codes.Unavailable,
		"DataLoss":           codes.DataLoss,
		"Unauthenticated":    codes.Unauthenticated,
	}

	code, ok := grpcCodes[reqCode]
	if !ok {
		return nil, status.Error(codes.Unknown, "Unknown status code for more information check https://chromium.googlesource.com/external/github.com/grpc/grpc/+/refs/tags/v1.21.4-pre1/doc/statuscodes.md")
	}

	return &pb.StatusResponse{Status: reqCode}, status.Error(code, "")
}
