package grpc

import (
	"context"
	"io"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"
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
	reflection.Register(srv)
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
	conn := newBufconnClient(t, srv)
	assertHealth(t, grpc_health_v1.NewHealthClient(conn), "", grpc_health_v1.HealthCheckResponse_SERVING)
	if _, err := status.NewStatusServiceClient(conn).Status(context.Background(), &status.StatusRequest{Code: "Ok"}); err != nil {
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

func histogramExemplars(t *testing.T, m *metrics, method string, code codes.Code) (uint64, []map[string]string) {
	t.Helper()

	var metric dto.Metric
	if err := m.histogram.WithLabelValues(method, code.String()).(prometheus.Metric).Write(&metric); err != nil {
		t.Fatalf("histogram write %v", err)
	}
	var exemplars []map[string]string
	for _, b := range metric.GetHistogram().GetBucket() {
		if e := b.GetExemplar(); e != nil {
			labels := map[string]string{}
			for _, l := range e.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			exemplars = append(exemplars, labels)
		}
	}
	return metric.GetHistogram().GetSampleCount(), exemplars
}

func listServices(t *testing.T, conn *grpc.ClientConn) {
	t.Helper()
	stream, err := grpc_reflection_v1.NewServerReflectionClient(conn).ServerReflectionInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerReflectionInfo returned error %v", err)
	}
	req := &grpc_reflection_v1.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{},
	}
	if err := stream.Send(req); err != nil {
		t.Fatalf("ServerReflectionInfo Send returned %v", err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("ServerReflectionInfo Recv returned %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("ServerReflectionInfo CloseSend returned %v", err)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("ServerReflectionInfo Recv returned %v, want EOF", err)
	}
}

func TestGrpcExemplars(t *testing.T) {
	calls := []struct {
		method string
		code   codes.Code
		traced bool
	}{
		{method: "/status.StatusService/Status", code: codes.OK, traced: true},
		{method: "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo", code: codes.OK, traced: true},
		{method: "/grpc.health.v1.Health/Watch", code: codes.Internal},
	}
	tests := []struct {
		name         string
		sampler      sdktrace.Sampler
		wantExemplar bool
	}{
		{name: "sampled", sampler: sdktrace.ParentBased(sdktrace.AlwaysSample()), wantExemplar: true},
		{name: "not sampled", sampler: sdktrace.NeverSample()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(tt.sampler), sdktrace.WithSyncer(exporter))
			reader := sdkmetric.NewManualReader()
			mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			prevTP, prevMP := otel.GetTracerProvider(), otel.GetMeterProvider()
			otel.SetTracerProvider(tp)
			otel.SetMeterProvider(mp)
			t.Cleanup(func() {
				otel.SetTracerProvider(prevTP)
				otel.SetMeterProvider(prevMP)
				_ = tp.Shutdown(context.Background())
				_ = mp.Shutdown(context.Background())
			})

			m := newMetrics(prometheus.NewRegistry())
			srv := newInstrumentedServer(m)
			conn := newBufconnClient(t, srv)
			if _, err := status.NewStatusServiceClient(conn).Status(context.Background(), &status.StatusRequest{Code: "Ok"}); err != nil {
				t.Fatalf("Status returned error %v", err)
			}
			stream, err := grpc_health_v1.NewHealthClient(conn).Watch(context.Background(), &grpc_health_v1.HealthCheckRequest{})
			if err != nil {
				t.Fatalf("Watch returned error %v", err)
			}
			if _, err := stream.Recv(); grpcstatus.Code(err) != codes.Internal {
				t.Fatalf("Watch Recv returned %v, want %v", err, codes.Internal)
			}
			listServices(t, conn)
			srv.GracefulStop()

			spans := map[string]trace.SpanContext{}
			for _, s := range exporter.GetSpans() {
				spans["/"+s.Name] = s.SpanContext
			}
			for _, c := range calls {
				count, exemplars := histogramExemplars(t, m, c.method, c.code)
				if count != 1 {
					t.Errorf("%s: sample count %d, want 1", c.method, count)
				}
				if _, ok := spans[c.method]; ok && !c.traced {
					t.Errorf("%s: got a span, want none", c.method)
				}
				if !tt.wantExemplar || !c.traced {
					if len(exemplars) != 0 {
						t.Errorf("%s: got exemplars %v, want none", c.method, exemplars)
					}
					continue
				}
				sc, ok := spans[c.method]
				if !ok || len(exemplars) != 1 {
					t.Fatalf("%s: got span %v and %d exemplars, want one each", c.method, ok, len(exemplars))
				}
				if got, want := exemplars[0]["trace_id"], sc.TraceID().String(); got != want {
					t.Errorf("%s: trace_id %q, want %q", c.method, got, want)
				}
				if got, want := exemplars[0]["span_id"], sc.SpanID().String(); got != want {
					t.Errorf("%s: span_id %q, want %q", c.method, got, want)
				}
			}

			var rm metricdata.ResourceMetrics
			if err := reader.Collect(context.Background(), &rm); err != nil {
				t.Fatal(err)
			}
			var found bool
			for _, sm := range rm.ScopeMetrics {
				for _, metric := range sm.Metrics {
					found = found || metric.Name == "rpc.server.call.duration"
				}
			}
			if !found {
				t.Error("rpc.server.call.duration not recorded")
			}
		})
	}
}
