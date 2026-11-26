package grpc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/echo"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

type logRecorder struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (r *logRecorder) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }

func (r *logRecorder) OnEmit(_ context.Context, record *sdklog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record.Clone())
	return nil
}

func (r *logRecorder) Shutdown(context.Context) error { return nil }

func (r *logRecorder) ForceFlush(context.Context) error { return nil }

func (r *logRecorder) find(msg string) (sdklog.Record, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := slices.IndexFunc(r.records, func(r sdklog.Record) bool { return r.Body().AsString() == msg })
	if i < 0 {
		return sdklog.Record{}, false
	}
	return r.records[i], true
}

func TestGrpcLogCorrelation(t *testing.T) {
	tests := []struct {
		name   string
		server func(*zap.Logger) echo.EchoServiceServer
		msg    string
	}{
		{
			name:   "handler",
			server: func(l *zap.Logger) echo.EchoServiceServer { return &echoServer{logger: l} },
			msg:    "received message body from client",
		},
		{
			name:   "panic recovery",
			server: func(*zap.Logger) echo.EchoServiceServer { return panicEchoServer{} },
			msg:    "gRPC handler panic",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
			previous := otel.GetTracerProvider()
			otel.SetTracerProvider(tp)
			t.Cleanup(func() {
				otel.SetTracerProvider(previous)
				_ = tp.Shutdown(context.Background())
			})

			recorder := &logRecorder{}
			lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(recorder))
			t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })
			var buf bytes.Buffer
			logger := zap.New(zapcore.NewTee(
				telemetry.NewTraceIDCore(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.Lock(zapcore.AddSync(&buf)), zap.DebugLevel)),
				otelzap.NewCore("test", otelzap.WithLoggerProvider(lp)),
			))

			s := &Server{logger: logger, config: &Config{}}
			srv := grpc.NewServer(s.serverOptions(newMetrics(prometheus.NewRegistry()))...)
			echo.RegisterEchoServiceServer(srv, tt.server(logger))
			_, _ = echo.NewEchoServiceClient(newBufconnClient(t, srv)).Echo(context.Background(), &echo.Message{Body: "test"})
			srv.GracefulStop()

			spans := exporter.GetSpans()
			if len(spans) != 1 {
				t.Fatalf("got %d spans, want 1", len(spans))
			}
			want := spans[0].SpanContext

			record, ok := recorder.find(tt.msg)
			if !ok {
				t.Fatalf("no exported %q record", tt.msg)
			}
			if record.TraceID() != want.TraceID() || record.SpanID() != want.SpanID() {
				t.Errorf("exported span context %s %s, want %s %s", record.TraceID(), record.SpanID(), want.TraceID(), want.SpanID())
			}

			var line map[string]any
			scanner := bufio.NewScanner(&buf)
			for scanner.Scan() {
				var l map[string]any
				if err := json.Unmarshal(scanner.Bytes(), &l); err != nil {
					t.Fatalf("invalid JSON log line %q: %v", scanner.Text(), err)
				}
				if l["msg"] == tt.msg {
					line = l
				}
			}
			if line["trace_id"] != want.TraceID().String() || line["span_id"] != want.SpanID().String() {
				t.Errorf("JSON line %v, want trace_id %s and span_id %s", line, want.TraceID(), want.SpanID())
			}
		})
	}
}
