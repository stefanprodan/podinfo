package grpc

import (
	"context"
	"runtime"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/pod"
	"github.com/stefanprodan/podinfo/pkg/environ"
	"github.com/stefanprodan/podinfo/pkg/version"
)

type podServer struct {
	pb.UnimplementedPodServiceServer
	config *Config
}

func (s *podServer) Pod(ctx context.Context, _ *pb.PodRequest) (*pb.PodResponse, error) {
	headers := map[string]*pb.Values{}
	md, _ := metadata.FromIncomingContext(ctx)
	for name, values := range environ.RedactHeaders(md) {
		// pseudo-headers are not request headers, and a client-supplied SPIFFE ID is never echoed
		if strings.HasPrefix(name, ":") || name == "x-spiffe-id" {
			continue
		}
		headers[name] = &pb.Values{Values: values}
	}

	return &pb.PodResponse{
		Hostname:     s.config.Hostname,
		Version:      version.VERSION,
		Revision:     version.REVISION,
		Goos:         runtime.GOOS,
		Goarch:       runtime.GOARCH,
		Runtime:      runtime.Version(),
		NumGoroutine: strconv.Itoa(runtime.NumGoroutine()),
		NumCpu:       strconv.Itoa(runtime.NumCPU()),
		Env:          environ.RedactedMap(),
		Headers:      headers,
	}, nil
}
