package grpc

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/echo"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/status"
)

type panicEchoServer struct {
	echo.UnimplementedEchoServiceServer
}

func (panicEchoServer) Echo(context.Context, *echo.Message) (*echo.Message, error) {
	panic("boom")
}

type panicHealthServer struct {
	*health.Server
}

func (panicHealthServer) Watch(*grpc_health_v1.HealthCheckRequest, grpc_health_v1.Health_WatchServer) error {
	panic("boom")
}

func newInstrumentedServer(m *metrics) *grpc.Server {
	s := &Server{logger: zap.NewNop(), config: &Config{}}
	srv := grpc.NewServer(s.serverOptions(m)...)
	echo.RegisterEchoServiceServer(srv, panicEchoServer{})
	status.RegisterStatusServiceServer(srv, &StatusServer{config: s.config, logger: s.logger})
	grpc_health_v1.RegisterHealthServer(srv, panicHealthServer{health.NewServer()})
	return srv
}

func histogramCount(t *testing.T, m *metrics, method string, code codes.Code) uint64 {
	t.Helper()

	var metric dto.Metric
	if err := m.histogram.WithLabelValues(method, code.String()).(prometheus.Metric).Write(&metric); err != nil {
		t.Fatalf("histogram write %v", err)
	}
	return metric.GetHistogram().GetSampleCount()
}

func TestGrpcMetrics(t *testing.T) {
	const method = "/status.StatusService/Status"
	m := newMetrics(prometheus.NewRegistry())
	client := status.NewStatusServiceClient(newBufconnClient(t, newInstrumentedServer(m)))

	_, err := client.Status(context.Background(), &status.StatusRequest{Code: "NotFound"})
	if grpcstatus.Code(err) != codes.NotFound {
		t.Fatalf("Status returned %v, want %v", err, codes.NotFound)
	}

	if got := testutil.ToFloat64(m.counter.WithLabelValues(codes.NotFound.String())); got != 1 {
		t.Errorf("grpc_requests_total{status=NotFound} = %v, want 1", got)
	}
	if got := histogramCount(t, m, method, codes.NotFound); got != 1 {
		t.Errorf("grpc_request_duration_seconds{method=%s,status=NotFound} count = %v, want 1", method, got)
	}
}

func TestGrpcUnaryRecovery(t *testing.T) {
	const method = "/echo.EchoService/Echo"
	m := newMetrics(prometheus.NewRegistry())
	conn := newBufconnClient(t, newInstrumentedServer(m))

	_, err := echo.NewEchoServiceClient(conn).Echo(context.Background(), &echo.Message{Body: "test"})
	if grpcstatus.Code(err) != codes.Internal {
		t.Fatalf("Echo returned %v, want %v", err, codes.Internal)
	}

	if got := histogramCount(t, m, method, codes.Internal); got != 1 {
		t.Errorf("grpc_request_duration_seconds{method=%s,status=Internal} count = %v, want 1", method, got)
	}

	assertHealth(t, grpc_health_v1.NewHealthClient(conn), "", grpc_health_v1.HealthCheckResponse_SERVING)
}

func TestGrpcStreamRecovery(t *testing.T) {
	const method = "/grpc.health.v1.Health/Watch"
	m := newMetrics(prometheus.NewRegistry())
	client := grpc_health_v1.NewHealthClient(newBufconnClient(t, newInstrumentedServer(m)))

	stream, err := client.Watch(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Watch returned error %v", err)
	}
	if _, err := stream.Recv(); grpcstatus.Code(err) != codes.Internal {
		t.Fatalf("Watch Recv returned %v, want %v", err, codes.Internal)
	}

	if got := histogramCount(t, m, method, codes.Internal); got != 1 {
		t.Errorf("grpc_request_duration_seconds{method=%s,status=Internal} count = %v, want 1", method, got)
	}

	assertHealth(t, client, "", grpc_health_v1.HealthCheckResponse_SERVING)
}

func TestGrpcTracing(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = tp.Shutdown(context.Background())
	})

	srv := newInstrumentedServer(newMetrics(prometheus.NewRegistry()))
	client := status.NewStatusServiceClient(newBufconnClient(t, srv))

	if _, err := client.Status(context.Background(), &status.StatusRequest{Code: "Ok"}); err != nil {
		t.Fatalf("Status returned error %v", err)
	}
	srv.GracefulStop()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if got, want := spans[0].Name, "status.StatusService/Status"; got != want {
		t.Errorf("span name %q, want %q", got, want)
	}
	if got := spans[0].SpanKind; got != trace.SpanKindServer {
		t.Errorf("span kind %v, want %v", got, trace.SpanKindServer)
	}
}
