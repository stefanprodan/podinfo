package grpc

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	storev1 "github.com/stefanprodan/podinfo/pkg/api/grpc/store/v1"
	"github.com/stefanprodan/podinfo/pkg/store"
)

func newStoreClient(t *testing.T, dataPath string) storev1.StoreServiceClient {
	t.Helper()

	srv := grpc.NewServer()
	storev1.RegisterStoreServiceServer(srv, &storeServer{
		store:  store.New(dataPath, noop.NewTracerProvider().Tracer("test")),
		logger: zap.NewNop(),
	})
	return storev1.NewStoreServiceClient(newBufconnClient(t, srv))
}

func TestGrpcStoreRoundTrip(t *testing.T) {
	client := newStoreClient(t, t.TempDir())
	content := []byte("hello world")

	res, err := client.Write(context.Background(), &storev1.WriteRequest{Content: content})
	if err != nil {
		t.Fatalf("Write returned error %v", err)
	}
	if want := store.Hash(content); res.GetHash() != want {
		t.Fatalf("Write returned hash %q, want %q", res.GetHash(), want)
	}

	read, err := client.Read(context.Background(), &storev1.ReadRequest{Hash: res.GetHash()})
	if err != nil {
		t.Fatalf("Read returned error %v", err)
	}
	if !bytes.Equal(read.GetContent(), content) {
		t.Errorf("Read returned %q, want %q", read.GetContent(), content)
	}
}

func TestGrpcStoreReadErrors(t *testing.T) {
	dataPath := t.TempDir()
	unreadable := strings.Repeat("b", 40)
	if err := os.Mkdir(filepath.Join(dataPath, unreadable), 0755); err != nil {
		t.Fatal(err)
	}
	client := newStoreClient(t, dataPath)

	tests := []struct {
		name string
		hash string
		code codes.Code
	}{
		{name: "invalid hash", hash: "../etc/passwd", code: codes.InvalidArgument},
		{name: "uppercase hash", hash: strings.Repeat("A", 40), code: codes.InvalidArgument},
		{name: "missing file", hash: strings.Repeat("a", 40), code: codes.NotFound},
		{name: "unreadable file", hash: unreadable, code: codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Read(context.Background(), &storev1.ReadRequest{Hash: tt.hash})
			wantCode(t, "Read", err, tt.code)
		})
	}
}

func TestGrpcStoreWriteError(t *testing.T) {
	client := newStoreClient(t, filepath.Join(t.TempDir(), "missing"))

	_, err := client.Write(context.Background(), &storev1.WriteRequest{Content: []byte("hello world")})
	wantCode(t, "Write", err, codes.Internal)
}

func TestGrpcStoreWriteMaxSize(t *testing.T) {
	s := &Server{config: &Config{}, logger: zap.NewNop()}
	srv := grpc.NewServer(s.serverOptions(newMetrics(prometheus.NewRegistry()))...)
	storev1.RegisterStoreServiceServer(srv, &storeServer{store: store.New(t.TempDir(), noop.NewTracerProvider().Tracer("test")), logger: s.logger})
	client := storev1.NewStoreServiceClient(newBufconnClient(t, srv))

	if _, err := client.Write(context.Background(), &storev1.WriteRequest{Content: make([]byte, 10<<20)}); err != nil {
		t.Fatalf("Write of 10 MiB returned error %v", err)
	}
	_, err := client.Write(context.Background(), &storev1.WriteRequest{Content: make([]byte, maxRecvMsgSize)})
	wantCode(t, "Write over the limit", err, codes.ResourceExhausted)
}
