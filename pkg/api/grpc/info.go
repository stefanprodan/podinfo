package grpc

import (
	"context"
	"runtime"
	"strconv"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/info"
	"github.com/stefanprodan/podinfo/pkg/version"
)

type infoServer struct {
	pb.UnimplementedInfoServiceServer
	config *Config
}

func (s *infoServer) Info(ctx context.Context, message *pb.InfoRequest) (*pb.InfoResponse, error) {
	return &pb.InfoResponse{
		Hostname:     s.config.Hostname,
		Version:      version.VERSION,
		Revision:     version.REVISION,
		Color:        s.config.UIColor,
		Logo:         s.config.UILogo,
		Message:      s.config.UIMessage,
		Goos:         runtime.GOOS,
		Goarch:       runtime.GOARCH,
		Runtime:      runtime.Version(),
		Numgoroutine: strconv.Itoa(runtime.NumGoroutine()),
		Numcpu:       strconv.Itoa(runtime.NumCPU()),
	}, nil
}
