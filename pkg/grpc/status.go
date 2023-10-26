package grpc

import (
	"context"

	pb "github.com/stefanprodan/podinfo/pkg/grpc/status"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"

)

type StatusServer struct {
	pb.UnimplementedStatusServiceServer
	config *Config
	logger *zap.Logger
}

// SayHello implements helloworld.GreeterServer

func (s *StatusServer) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	reqCode := req.GetCode()
	// return &pb.StatusResponse{Status: reqCode}, nil

	// type Code Code.code
	grpcCodes := map[string]codes.Code{
		"Ok": codes.OK,
		"Canceled": codes.Canceled,
		"Unknown": codes.Unknown,
		"InvalidArgument": codes.InvalidArgument,
		"DeadlineExceeded": codes.DeadlineExceeded,
		"NotFound": codes.NotFound,
		"AlreadyExists": codes.AlreadyExists,
		"PermissionDenied": codes.PermissionDenied,
		"ResourceExhausted": codes.ResourceExhausted,
		"FailedPrecondition": codes.FailedPrecondition,
		"Aborted": codes.Aborted,
		"OutOfRange": codes.OutOfRange,
		"Unimplemented": codes.Unimplemented,
		"Internal": codes.Internal,
		"Unavailable": codes.Unavailable,
		"DataLoss": codes.DataLoss,
		"Unauthenticated": codes.Unauthenticated,
	}

	// try to access the map with the request code string as key. If the key is not found, return an error
	// if the key is found, return the grpc status code
	code, ok := grpcCodes[reqCode]
	s.logger.Info(string(code))
	if !ok {
		return nil, status.Error(codes.Unknown, "Unknown status code")
	}
	
	return &pb.StatusResponse{Status: reqCode}, status.Error(code, "")
}
