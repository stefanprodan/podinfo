package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stefanprodan/podinfo/pkg/store"
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

func newLoggingServer(t *testing.T, level zapcore.Level) (*Server, *logRecorder, *bytes.Buffer) {
	t.Helper()
	recorder := &logRecorder{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(recorder))
	t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })
	var buf bytes.Buffer
	srv := NewMockServer()
	srv.logger = zap.New(zapcore.NewTee(
		telemetry.NewTraceIDCore(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.Lock(zapcore.AddSync(&buf)), level)),
		otelzap.NewCore("test", otelzap.WithLoggerProvider(lp)),
	), zap.IncreaseLevel(level))
	srv.store = store.New(filepath.Join(t.TempDir(), "missing"), telemetry.Tracer())
	return srv, recorder, &buf
}

const testPayload = `{"secret":"payload-4bf92f35"}`

func TestLogCorrelation(t *testing.T) {
	tests := []struct {
		name    string
		tracing bool
		cache   bool
		method  string
		path    string
		body    string
		msg     string
	}{
		{name: "request started", tracing: true, method: "GET", path: "/env", msg: "request started"},
		{name: "payload received", tracing: true, method: "POST", path: "/api/echo", body: testPayload, msg: "payload received"},
		{name: "store read failed", tracing: true, method: "GET", path: "/store/" + store.Hash([]byte("missing")), msg: "reading file failed"},
		{name: "cache set failed", tracing: true, cache: true, method: "POST", path: "/cache/key1", body: "value1", msg: "cache set failed"},
		{name: "remote span without tracing", method: "GET", path: "/env", msg: "request started"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var spans *tracetest.InMemoryExporter
			if tt.tracing {
				spans = setupTracing(t)
			} else {
				setGlobalTracing(t, noop.NewTracerProvider())
			}

			srv, recorder, buf := newLoggingServer(t, zapcore.DebugLevel)
			if tt.cache {
				m := miniredis.RunT(t)
				opts, _, err := newCacheOptions(&Config{CacheServer: "redis://" + m.Addr()}, zap.NewNop())
				if err != nil {
					t.Fatal(err)
				}
				srv.openCache(opts)
				t.Cleanup(func() { _ = srv.Close() })
				m.SetError("LOADING")
			}
			srv.setupHandler()

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Traceparent", testTraceparent)
			srv.handler.ServeHTTP(httptest.NewRecorder(), req)

			want := trace.SpanContextFromContext(propagation.TraceContext{}.Extract(context.Background(), propagation.HeaderCarrier(req.Header)))
			if tt.tracing {
				servers := spansOfKind(spans.GetSpans(), trace.SpanKindServer)
				if len(servers) != 1 {
					t.Fatalf("got %d server spans, want 1", len(servers))
				}
				want = servers[0].SpanContext
			}

			record, ok := recorder.find(tt.msg)
			if !ok {
				t.Fatalf("no exported %q record", tt.msg)
			}
			if record.TraceID() != want.TraceID() || record.SpanID() != want.SpanID() || record.TraceFlags() != want.TraceFlags() {
				t.Errorf("exported span context %s %s %s, want %s %s %s",
					record.TraceID(), record.SpanID(), record.TraceFlags(), want.TraceID(), want.SpanID(), want.TraceFlags())
			}

			var line map[string]any
			scanner := bufio.NewScanner(buf)
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

func TestLogPayloadInfoLevel(t *testing.T) {
	setupTracing(t)
	srv, recorder, buf := newLoggingServer(t, zapcore.InfoLevel)
	srv.setupHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/echo", strings.NewReader(testPayload))
	srv.handler.ServeHTTP(httptest.NewRecorder(), req)

	if _, ok := recorder.find("payload received"); ok {
		t.Error("payload received exported at info level")
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	for _, r := range recorder.records {
		r.WalkAttributes(func(kv attribute.KeyValue) bool {
			if strings.Contains(kv.Value.String(), "payload-4bf92f35") {
				t.Errorf("record %q exports the request body in %s", r.Body().AsString(), kv.Key)
			}
			return true
		})
	}
	if strings.Contains(buf.String(), "payload-4bf92f35") {
		t.Errorf("JSON output has the request body: %s", buf.String())
	}
}
