package telemetry

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	lognoop "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/stefanprodan/podinfo/pkg/version"
)

var otelEnv = []string{
	"OTEL_SDK_DISABLED",
	"OTEL_SERVICE_NAME",
	"OTEL_RESOURCE_ATTRIBUTES",
	"OTEL_PROPAGATORS",
	"OTEL_TRACES_EXPORTER",
	"OTEL_LOGS_EXPORTER",
	"OTEL_EXPORTER_OTLP_ENDPOINT",
	"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
	"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
	"OTEL_EXPORTER_OTLP_PROTOCOL",
	"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
	"OTEL_EXPORTER_OTLP_LOGS_PROTOCOL",
}

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range otelEnv {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func resetGlobals(t *testing.T) {
	tp := otel.GetTracerProvider()
	p := otel.GetTextMapPropagator()
	lp := otel.GetLoggerProvider()
	eh := otel.GetErrorHandler()
	otel.SetTracerProvider(noop.NewTracerProvider())
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	otel.SetLoggerProvider(lognoop.NewLoggerProvider())
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(error) {}))
	t.Cleanup(func() {
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(p)
		otel.SetLoggerProvider(lp)
		otel.SetErrorHandler(eh)
	})
}

func setup(t *testing.T, serviceName string) (*Telemetry, error) {
	t.Helper()
	tel, err := Setup(context.Background(), serviceName, zap.NewNop())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = tel.Shutdown(ctx)
	})
	return tel, err
}

func tracesEnabled() bool {
	_, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	return ok
}

func TestSetupSignals(t *testing.T) {
	const endpoint = "http://127.0.0.1:4318"
	tests := []struct {
		name       string
		env        map[string]string
		wantTraces bool
		wantLogs   bool
	}{
		{name: "no endpoint"},
		{
			name: "sdk disabled",
			env:  map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": endpoint, "OTEL_SDK_DISABLED": "TRUE"},
		},
		{
			name:       "endpoint",
			env:        map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": endpoint},
			wantTraces: true,
			wantLogs:   true,
		},
		{
			name:       "traces endpoint",
			env:        map[string]string{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": endpoint + "/v1/traces"},
			wantTraces: true,
		},
		{
			name:     "logs endpoint",
			env:      map[string]string{"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": endpoint + "/v1/logs"},
			wantLogs: true,
		},
		{
			name:     "traces exporter none",
			env:      map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": endpoint, "OTEL_TRACES_EXPORTER": "none"},
			wantLogs: true,
		},
		{
			name:       "logs exporter none",
			env:        map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": endpoint, "OTEL_LOGS_EXPORTER": "none"},
			wantTraces: true,
		},
		{
			name:       "console traces exporter",
			env:        map[string]string{"OTEL_TRACES_EXPORTER": "console"},
			wantTraces: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobals(t)
			setEnv(t, tt.env)

			tel, err := setup(t, "")
			if err != nil {
				t.Fatal(err)
			}

			if got := tracesEnabled(); got != tt.wantTraces {
				t.Errorf("traces enabled: got %v, want %v", got, tt.wantTraces)
			}
			_, span := Tracer().Start(context.Background(), "test")
			span.End()
			if got := span.SpanContext().IsValid(); got != tt.wantTraces {
				t.Errorf("recording span: got %v, want %v", got, tt.wantTraces)
			}

			if got := tel.LoggerProvider() != nil; got != tt.wantLogs {
				t.Errorf("logs enabled: got %v, want %v", got, tt.wantLogs)
			}
			if got := otel.GetLoggerProvider() == tel.LoggerProvider(); got != tt.wantLogs {
				t.Errorf("global logger provider set: got %v, want %v", got, tt.wantLogs)
			}
		})
	}
}

func TestSetupSDKDisabledLeavesPropagator(t *testing.T) {
	resetGlobals(t)
	setEnv(t, map[string]string{"OTEL_SDK_DISABLED": "true"})

	if _, err := setup(t, ""); err != nil {
		t.Fatal(err)
	}
	if fields := otel.GetTextMapPropagator().Fields(); len(fields) != 0 {
		t.Errorf("propagator fields: got %v, want none", fields)
	}
}

func TestSetupPropagators(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "default",
			want: []string{"baggage", "traceparent", "tracestate"},
		},
		{
			name: "b3 jaeger ottrace",
			env:  map[string]string{"OTEL_PROPAGATORS": "b3,jaeger,ottrace"},
			want: []string{"b3", "ot-tracer-sampled", "ot-tracer-spanid", "ot-tracer-traceid", "uber-trace-id"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobals(t)
			setEnv(t, tt.env)

			if _, err := setup(t, ""); err != nil {
				t.Fatal(err)
			}
			if got := slices.Sorted(slices.Values(otel.GetTextMapPropagator().Fields())); !slices.Equal(got, tt.want) {
				t.Errorf("propagator fields: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetupInvalidProtocol(t *testing.T) {
	for _, key := range []string{"OTEL_EXPORTER_OTLP_PROTOCOL", "OTEL_EXPORTER_OTLP_LOGS_PROTOCOL"} {
		t.Run(key, func(t *testing.T) {
			resetGlobals(t)
			setEnv(t, map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:4318", key: "thrift"})

			tel, err := setup(t, "")
			if err == nil {
				t.Fatal("expected an error")
			}
			if tracesEnabled() {
				t.Error("expected the global tracer provider to be left untouched")
			}
			if tel.LoggerProvider() != nil {
				t.Error("expected no logger provider")
			}
			if _, ok := otel.GetLoggerProvider().(lognoop.LoggerProvider); !ok {
				t.Error("expected the global logger provider to be left untouched")
			}
		})
	}
}

func TestNewResource(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		serviceName string
		want        string
	}{
		{name: "default", want: "podinfo"},
		{
			name: "resource attributes",
			env:  map[string]string{"OTEL_RESOURCE_ATTRIBUTES": "service.name=from-attrs"},
			want: "from-attrs",
		},
		{
			name:        "argument over resource attributes",
			env:         map[string]string{"OTEL_RESOURCE_ATTRIBUTES": "service.name=from-attrs"},
			serviceName: "from-arg",
			want:        "from-arg",
		},
		{
			name: "env over argument",
			env: map[string]string{
				"OTEL_RESOURCE_ATTRIBUTES": "service.name=from-attrs",
				"OTEL_SERVICE_NAME":        "from-env",
			},
			serviceName: "from-arg",
			want:        "from-env",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.env)

			res, err := newResource(context.Background(), tt.serviceName)
			if err != nil {
				t.Fatal(err)
			}
			if got, _ := res.Set().Value("service.name"); got.AsString() != tt.want {
				t.Errorf("service.name: got %q, want %q", got.AsString(), tt.want)
			}
		})
	}
}

func TestNewResourceAttributes(t *testing.T) {
	setEnv(t, map[string]string{"OTEL_RESOURCE_ATTRIBUTES": "deployment.environment.name=test,host.name=from-attrs"})

	res, err := newResource(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	set := res.Set()

	want := map[attribute.Key]string{
		"service.version":             version.VERSION,
		"deployment.environment.name": "test",
		"host.name":                   "from-attrs",
		"telemetry.sdk.language":      "go",
		"process.runtime.name":        "go",
	}
	for k, v := range want {
		if got, _ := set.Value(k); got.AsString() != v {
			t.Errorf("%s: got %q, want %q", k, got.AsString(), v)
		}
	}

	for _, k := range []attribute.Key{
		"service.instance.id",
		"process.pid",
		"process.executable.name",
		"process.runtime.version",
		"process.runtime.description",
		"os.type",
	} {
		if !set.HasValue(k) {
			t.Errorf("%s: missing", k)
		}
	}

	for _, k := range []attribute.Key{"process.command_args", "process.owner"} {
		if set.HasValue(k) {
			t.Errorf("%s: unexpected", k)
		}
	}
}

type collector struct {
	*httptest.Server
	mu       sync.Mutex
	requests map[string][][]byte
}

func newCollector(t *testing.T) *collector {
	c := &collector{requests: map[string][][]byte{}}
	c.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.requests[r.URL.Path] = append(c.requests[r.URL.Path], body)
		c.mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	t.Cleanup(c.Close)
	return c
}

func (c *collector) received(path string) [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requests[path]
}

func TestShutdownFlushes(t *testing.T) {
	resetGlobals(t)
	c := newCollector(t)
	setEnv(t, map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": c.URL})

	tel, err := setup(t, "podinfo-test")
	if err != nil {
		t.Fatal(err)
	}

	_, span := Tracer().Start(context.Background(), "test")
	span.End()
	zap.New(otelzap.NewCore("test", otelzap.WithLoggerProvider(tel.LoggerProvider()))).Info("hello")

	if err := tel.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	traces := c.received("/v1/traces")
	if len(traces) == 0 {
		t.Fatal("no request on /v1/traces")
	}
	var req coltracepb.ExportTraceServiceRequest
	if err := proto.Unmarshal(traces[0], &req); err != nil {
		t.Fatal(err)
	}
	var serviceName string
	for _, kv := range req.GetResourceSpans()[0].GetResource().GetAttributes() {
		if kv.GetKey() == "service.name" {
			serviceName = kv.GetValue().GetStringValue()
		}
	}
	if serviceName != "podinfo-test" {
		t.Errorf("service.name: got %q, want %q", serviceName, "podinfo-test")
	}

	if logs := c.received("/v1/logs"); len(logs) == 0 || !bytes.Contains(logs[0], []byte("hello")) {
		t.Error("log record not received on /v1/logs")
	}

	if err := tel.Shutdown(context.Background()); err != nil {
		t.Errorf("second Shutdown: %v", err)
	}
}

func TestExitHookFlushes(t *testing.T) {
	resetGlobals(t)
	c := newCollector(t)
	setEnv(t, map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": c.URL})

	tel, err := setup(t, "")
	if err != nil {
		t.Fatal(err)
	}

	code := -1
	logger := zap.New(
		otelzap.NewCore("test", otelzap.WithLoggerProvider(tel.LoggerProvider())),
		zap.WithFatalHook(tel.ExitHook(time.Second, func(c int) { code = c })),
	)
	logger.Fatal("boom")

	if code != 1 {
		t.Errorf("exit code: got %d, want 1", code)
	}
	if logs := c.received("/v1/logs"); len(logs) == 0 || !bytes.Contains(logs[0], []byte("boom")) {
		t.Error("fatal record not received on /v1/logs")
	}
}
