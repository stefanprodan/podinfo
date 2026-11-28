package grpc

import (
	"context"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	echov1 "github.com/stefanprodan/podinfo/pkg/api/grpc/echo/v1"
	"github.com/stefanprodan/podinfo/pkg/spiffe"
	"github.com/stefanprodan/podinfo/pkg/spiffe/spiffetest"
	"github.com/stefanprodan/podinfo/pkg/version"
)

// fakeEchoBackend answers with a prefixed body and records the request metadata.
type fakeEchoBackend struct {
	echov1.UnimplementedEchoServiceServer
	block bool

	mu sync.Mutex
	md metadata.MD
}

func (b *fakeEchoBackend) Echo(ctx context.Context, req *echov1.EchoRequest) (*echov1.EchoResponse, error) {
	if b.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	md, _ := metadata.FromIncomingContext(ctx)
	b.mu.Lock()
	b.md = md
	b.mu.Unlock()
	return &echov1.EchoResponse{Body: "backend " + req.GetBody()}, nil
}

func (b *fakeEchoBackend) metadata(key string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Join(b.md.Get(key), ",")
}

func newEchoBackend(t *testing.T, url string, impl echov1.EchoServiceServer) echoBackend {
	t.Helper()
	srv := grpc.NewServer()
	echov1.RegisterEchoServiceServer(srv, impl)
	return echoBackend{url: url, client: echov1.NewEchoServiceClient(newBufconnClient(t, srv))}
}

func newEchoClient(t *testing.T, timeout time.Duration, backends ...echoBackend) echov1.EchoServiceClient {
	t.Helper()
	s := &Server{logger: zap.NewNop(), config: &Config{ClientTimeout: timeout}, backends: backends}
	srv, _ := s.newServer()
	return echov1.NewEchoServiceClient(newBufconnClient(t, srv))
}

func TestGrpcEcho(t *testing.T) {
	client := newEchoClient(t, 0)

	res, err := client.Echo(context.Background(), &echov1.EchoRequest{Body: "test123-test"})
	if err != nil {
		t.Fatalf("Echo returned error %v", err)
	}
	if res.GetBody() != "test123-test" {
		t.Errorf("body = %q, want %q", res.GetBody(), "test123-test")
	}
	if len(res.GetBackendResponses()) != 0 {
		t.Errorf("got backend responses %v without backends", res.GetBackendResponses())
	}
}

func TestGrpcEchoBackends(t *testing.T) {
	ok := &fakeEchoBackend{}
	client := newEchoClient(t, 5*time.Second,
		newEchoBackend(t, "backend-1:9999", ok),
		newEchoBackend(t, "backend-2:9999", echov1.UnimplementedEchoServiceServer{}))

	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-request-id", "req-123")
	res, err := client.Echo(ctx, &echov1.EchoRequest{Body: "ping"})
	if err != nil {
		t.Fatalf("Echo returned error %v", err)
	}
	if res.GetBody() != "ping" {
		t.Errorf("body = %q, want %q", res.GetBody(), "ping")
	}

	for key, want := range map[string]string{
		"x-request-id":   "req-123",
		"x-api-version":  version.VERSION,
		"x-api-revision": version.REVISION,
	} {
		if got := ok.metadata(key); got != want {
			t.Errorf("backend metadata %s = %q, want %q", key, got, want)
		}
	}

	responses := res.GetBackendResponses()
	if len(responses) != 2 {
		t.Fatalf("got %d backend responses, want 2: %v", len(responses), responses)
	}
	if r := responses[0]; r.GetUrl() != "backend-1:9999" || r.GetBody() != "backend ping" || r.GetError() != "" {
		t.Errorf("responses[0] = %v, want the backend-1 body and no error", r)
	}
	if r := responses[1]; r.GetUrl() != "backend-2:9999" || r.GetBody() != "" || !strings.Contains(r.GetError(), "code = Unimplemented") {
		t.Errorf("responses[1] = %v, want the backend-2 Unimplemented error", r)
	}
}

func TestGrpcEchoBackends_NoRequestID(t *testing.T) {
	ok := &fakeEchoBackend{}
	client := newEchoClient(t, 5*time.Second, newEchoBackend(t, "backend:9999", ok))

	if _, err := client.Echo(context.Background(), &echov1.EchoRequest{Body: "ping"}); err != nil {
		t.Fatalf("Echo returned error %v", err)
	}
	if got := ok.metadata("x-request-id"); got != "" {
		t.Errorf("x-request-id %q forwarded without a request ID", got)
	}
}

func TestGrpcEchoBackends_Timeout(t *testing.T) {
	client := newEchoClient(t, 50*time.Millisecond, newEchoBackend(t, "backend:9999", &fakeEchoBackend{block: true}))

	res, err := client.Echo(context.Background(), &echov1.EchoRequest{Body: "ping"})
	if err != nil {
		t.Fatalf("Echo returned error %v", err)
	}
	responses := res.GetBackendResponses()
	if len(responses) != 1 || !strings.Contains(responses[0].GetError(), "code = DeadlineExceeded") {
		t.Errorf("backend responses %v, want one DeadlineExceeded error", responses)
	}
}

// listenAndServe starts the server on a free port and returns its address.
func listenAndServe(t *testing.T, s *Server) (string, *grpc.Server) {
	t.Helper()
	s.config.Host = "127.0.0.1"
	s.config.Port = freePort(t)
	srv, _ := s.ListenAndServe()
	t.Cleanup(func() {
		srv.Stop()
		_ = s.Close()
	})
	return net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port)), srv
}

func TestGrpcEchoBackendURL(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previousProvider, previousPropagator := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
		_ = tp.Shutdown(context.Background())
	})

	backendAddr, backendSrv := listenAndServe(t, NewServer(&Config{}, zap.NewNop()))
	frontendAddr, frontendSrv := listenAndServe(t, NewServer(&Config{
		BackendURL:    []string{"grpc://" + backendAddr},
		ClientTimeout: 5 * time.Second,
	}, zap.NewNop()))

	conn, err := grpc.NewClient(frontendAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	res, err := echov1.NewEchoServiceClient(conn).Echo(context.Background(), &echov1.EchoRequest{Body: "ping"})
	if err != nil {
		t.Fatalf("Echo returned error %v", err)
	}
	responses := res.GetBackendResponses()
	if len(responses) != 1 {
		t.Fatalf("got %d backend responses, want 1", len(responses))
	}
	if r := responses[0]; r.GetUrl() != "grpc://"+backendAddr || r.GetBody() != "ping" || r.GetError() != "" {
		t.Errorf("response = %v, want %q from grpc://%s", r, "ping", backendAddr)
	}

	frontendSrv.GracefulStop()
	backendSrv.GracefulStop()

	spans := exporter.GetSpans()
	kinds := map[trace.SpanKind]int{}
	for _, span := range spans {
		kinds[span.SpanKind]++
		if span.Name != "echo.v1.EchoService/Echo" {
			t.Errorf("span name %q, want echo.v1.EchoService/Echo", span.Name)
		}
		if span.SpanContext.TraceID() != spans[0].SpanContext.TraceID() {
			t.Errorf("span %s is in trace %s, want %s", span.SpanKind, span.SpanContext.TraceID(), spans[0].SpanContext.TraceID())
		}
	}
	if kinds[trace.SpanKindServer] != 2 || kinds[trace.SpanKindClient] != 1 {
		t.Errorf("span kinds %v, want 2 server spans and 1 client span", kinds)
	}
}

func TestGrpcEchoBackendSPIFFE(t *testing.T) {
	ca := spiffetest.NewCA(t)
	socket := spiffetest.SocketPath(t)
	spiffetest.StartWorkloadAPI(t, socket, ca, ca.NewSVID(t, serverID))
	source := spiffe.NewSource("unix://"+socket, nil, zap.NewNop(), spiffetest.FastBackoff())
	t.Cleanup(func() { _ = source.Close() })

	backend := NewServer(&Config{TLS: true}, zap.NewNop())
	backend.SetSPIFFESource(source)
	backendAddr, _ := listenAndServe(t, backend)
	if _, err := source.Wait(); err != nil {
		t.Fatal(err)
	}

	echo := func(t *testing.T, scheme string, config *Config) *echov1.BackendResponse {
		t.Helper()
		config.BackendURL = []string{scheme + "://" + backendAddr}
		config.ClientTimeout = 5 * time.Second
		s := NewServer(config, zap.NewNop())
		s.SetSPIFFESource(source)
		if err := s.dialBackends(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		srv, _ := s.newServer()
		res, err := echov1.NewEchoServiceClient(newBufconnClient(t, srv)).Echo(context.Background(), &echov1.EchoRequest{Body: "ping"})
		if err != nil {
			t.Fatalf("Echo returned error %v", err)
		}
		if len(res.GetBackendResponses()) != 1 {
			t.Fatalf("got %d backend responses, want 1", len(res.GetBackendResponses()))
		}
		return res.GetBackendResponses()[0]
	}

	t.Run("trust domain", func(t *testing.T) {
		if r := echo(t, "grpcs", &Config{SPIFFEBackendMTLS: true}); r.GetBody() != "ping" || r.GetError() != "" {
			t.Errorf("response = %v, want the body", r)
		}
	})

	t.Run("backend ID", func(t *testing.T) {
		r := echo(t, "grpcs", &Config{SPIFFEBackendMTLS: true, SPIFFEBackendIDs: []string{serverID.String()}})
		if r.GetBody() != "ping" || r.GetError() != "" {
			t.Errorf("response = %v, want the body", r)
		}
	})

	failures := []struct {
		name   string
		scheme string
		config Config
		want   string
	}{
		{
			name:   "other backend ID",
			scheme: "grpcs",
			config: Config{SPIFFEBackendMTLS: true, SPIFFEBackendIDs: []string{otherID.String()}},
			want:   "unexpected ID",
		},
		{name: "system roots", scheme: "grpcs", want: "failed to verify certificate"},
		{name: "plaintext", scheme: "grpc", config: Config{SPIFFEBackendMTLS: true}, want: "code = Unavailable"},
	}
	for _, tt := range failures {
		t.Run(tt.name, func(t *testing.T) {
			if r := echo(t, tt.scheme, &tt.config); r.GetBody() != "" || !strings.Contains(r.GetError(), tt.want) {
				t.Errorf("response = %v, want an error with %q", r, tt.want)
			}
		})
	}
}

func TestGrpcEchoBackendChain(t *testing.T) {
	leafAddr, _ := listenAndServe(t, NewServer(&Config{}, zap.NewNop()))
	middleAddr, _ := listenAndServe(t, NewServer(&Config{BackendURL: []string{"grpc://" + leafAddr}}, zap.NewNop()))
	frontendAddr, _ := listenAndServe(t, NewServer(&Config{BackendURL: []string{"grpc://" + middleAddr}}, zap.NewNop()))

	conn, err := grpc.NewClient(frontendAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// The middle server answers with the body and the leaf response, over the 10 MiB request limit.
	body := strings.Repeat("a", 6<<20)
	res, err := echov1.NewEchoServiceClient(conn).Echo(context.Background(), &echov1.EchoRequest{Body: body})
	if err != nil {
		t.Fatalf("Echo returned error %v", err)
	}
	responses := res.GetBackendResponses()
	if len(responses) != 1 || responses[0].GetError() != "" || responses[0].GetBody() != body {
		t.Fatalf("got %d backend responses with error %q, want the body from the middle server", len(responses), responses[0].GetError())
	}
}

func TestGrpcBackendConfig(t *testing.T) {
	const wantURL = "want grpc://host:port or grpcs://host:port"
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name:   "SPIFFE mTLS without a source",
			config: Config{BackendURL: []string{"grpcs://backend:9999"}, SPIFFEBackendMTLS: true},
			want:   "--spiffe-backend-mtls requires --spiffe-endpoint-socket",
		},
		{
			name:   "invalid SPIFFE ID",
			config: Config{BackendURL: []string{"grpcs://backend:9999"}, SPIFFEBackendMTLS: true, SPIFFEBackendIDs: []string{"backend"}},
			want:   "--spiffe-backend-ids",
		},
		{name: "no scheme", config: Config{BackendURL: []string{"backend:9999"}}, want: wantURL},
		{name: "http scheme", config: Config{BackendURL: []string{"http://backend:9999"}}, want: wantURL},
		{name: "no host", config: Config{BackendURL: []string{"grpc://:9999"}}, want: wantURL},
		{name: "no port", config: Config{BackendURL: []string{"grpc://backend"}}, want: wantURL},
		{name: "port out of range", config: Config{BackendURL: []string{"grpc://backend:99999"}}, want: wantURL},
		{name: "path", config: Config{BackendURL: []string{"grpc://backend:9999/echo"}}, want: wantURL},
		{name: "credentials", config: Config{BackendURL: []string{"grpc://user:s3cret@backend:9999"}}, want: wantURL},
		{name: "query", config: Config{BackendURL: []string{"grpc://backend:9999?token=s3cret"}}, want: wantURL},
		{name: "second URL", config: Config{BackendURL: []string{"grpc://backend:9999", "s3cret"}}, want: "at index 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(&tt.config, zap.NewNop())
			if tt.config.SPIFFEBackendIDs != nil {
				s.SetSPIFFESource(spiffe.NewSource("unix:///nonexistent.sock", nil, zap.NewNop()))
				t.Cleanup(func() { _ = s.spiffe.Close() })
			}
			err := s.dialBackends()
			t.Cleanup(func() { _ = s.Close() })
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got error %v, want %q", err, tt.want)
			}
			if strings.Contains(err.Error(), "s3cret") {
				t.Errorf("error %q holds the URL", err)
			}
		})
	}
}

func TestGrpcBackendTarget(t *testing.T) {
	// Hosts named like a gRPC resolver scheme must stay hosts.
	for backend, want := range map[string]string{
		"grpc://dns:9999":        "dns:///dns:9999",
		"grpc://unix:9999":       "dns:///unix:9999",
		"GRPCS://Backend.ns:443": "dns:///Backend.ns:443",
		"grpc://[::1]:9999":      "dns:///[::1]:9999",
	} {
		s := NewServer(&Config{BackendURL: []string{backend}}, zap.NewNop())
		if err := s.dialBackends(); err != nil {
			t.Errorf("%s: %v", backend, err)
			continue
		}
		if got := s.backendConns[0].Target(); got != want {
			t.Errorf("%s: target %q, want %q", backend, got, want)
		}
		_ = s.Close()
	}
}
