package grpc

import (
	"context"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/header/v1"
	"github.com/stefanprodan/podinfo/pkg/environ"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const spiffeIDKey = "x-spiffe-id"

type headerServer struct {
	pb.UnimplementedHeaderServiceServer
	config *Config
	logger *zap.Logger
}

func (s *headerServer) Header(ctx context.Context, _ *pb.HeaderRequest) (*pb.HeaderResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.DataLoss, "failed to get metadata")
	}

	// Headers are returned as name=value strings: their names are not part of the proto contract.
	res := []string{}
	for i, e := range echoedMetadata(ctx, md) {
		res = append(res, i+"="+strings.Join(e, ","))
	}

	return &pb.HeaderResponse{Headers: res}, nil
}

// echoedMetadata returns the request metadata with credentials masked and
// x-spiffe-id set only to the verified peer ID, never to a client-supplied value.
func echoedMetadata(ctx context.Context, md metadata.MD) map[string][]string {
	headers := environ.RedactHeaders(md)
	delete(headers, spiffeIDKey)
	if id, ok := peerSPIFFEID(ctx); ok {
		headers[spiffeIDKey] = []string{id.String()}
	}
	return headers
}

// peerSPIFFEID returns the SPIFFE ID of the TLS client certificate, if any.
func peerSPIFFEID(ctx context.Context) (spiffeid.ID, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return spiffeid.ID{}, false
	}
	info, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(info.State.PeerCertificates) == 0 {
		return spiffeid.ID{}, false
	}
	id, err := x509svid.IDFromCert(info.State.PeerCertificates[0])
	if err != nil {
		return spiffeid.ID{}, false
	}
	return id, true
}
