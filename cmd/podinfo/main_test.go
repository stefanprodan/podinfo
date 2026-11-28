package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/stefanprodan/podinfo/pkg/api/grpc"
	podinfohttp "github.com/stefanprodan/podinfo/pkg/api/http"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

const runMainEnv = "TEST_RUN_MAIN"

// TestMain runs main in place of the tests when the test binary is executed
// by runMain.
func TestMain(m *testing.M) {
	if os.Getenv(runMainEnv) == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

func runMain(t *testing.T, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	args = append([]string{"--host=127.0.0.1", "--port=0", "--artifact-path=", "--data-path=" + dir, "--config-path=" + dir}, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], args...) //nolint:gosec // re-executes the test binary
	cmd.Env = []string{runMainEnv + "=1", "OTEL_SDK_DISABLED=true"}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestGRPCTLSRequiresPort(t *testing.T) {
	out, err := runMain(t, "--grpc-tls")
	if exitErr, ok := errors.AsType[*exec.ExitError](err); !ok || exitErr.ExitCode() != 1 {
		t.Errorf("got error %v, want exit code 1", err)
	}
	if want := "--grpc-tls requires --grpc-port"; !strings.Contains(out, want) {
		t.Errorf("got output %q, want %q", out, want)
	}
}

func TestResetInvalidPorts(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("PODINFO_PORT", "tcp://10.0.0.1:9898")
	t.Setenv("PODINFO_SECURE_PORT", "9899")
	t.Setenv("PODINFO_GRPC_PORT", "tcp://10.0.0.1:9999")
	t.Setenv("PODINFO_PORT_METRICS", "tcp://10.0.0.1:9797")

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Int("port", 9898, "")
	fs.Int("secure-port", 0, "")
	fs.Int("grpc-port", 0, "")
	fs.Int("port-metrics", 0, "")
	if err := fs.Parse([]string{"--port-metrics=9797"}); err != nil {
		t.Fatal(err)
	}
	if err := viper.BindPFlags(fs); err != nil {
		t.Fatal(err)
	}
	viper.SetEnvPrefix("PODINFO")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	resetInvalidPorts(fs, "port", "secure-port", "grpc-port", "port-metrics")

	var grpcCfg grpc.Config
	if err := viper.Unmarshal(&grpcCfg); err != nil {
		t.Fatal(err)
	}
	var srvCfg podinfohttp.Config
	if err := viper.Unmarshal(&srvCfg); err != nil {
		t.Fatal(err)
	}
	if srvCfg.Port != "9898" {
		t.Errorf("port: got %q, want 9898", srvCfg.Port)
	}
	if srvCfg.SecurePort != "9899" {
		t.Errorf("secure-port: got %q, want 9899", srvCfg.SecurePort)
	}
	if grpcCfg.Port != 0 {
		t.Errorf("grpc-port: got %d, want 0", grpcCfg.Port)
	}
	if srvCfg.PortMetrics != 9797 {
		t.Errorf("port-metrics: got %d, want 9797", srvCfg.PortMetrics)
	}
}

func TestParseLevel(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error", "fatal", "panic"} {
		if _, err := parseLevel(level); err != nil {
			t.Errorf("parseLevel(%q) returned %v", level, err)
		}
	}

	if _, err := parseLevel("verbose"); err == nil {
		t.Error("parseLevel(\"verbose\") returned no error")
	}
}

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

func (r *logRecorder) get() []sdklog.Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.records)
}

func mustParseLevel(t *testing.T, level string) zap.AtomicLevel {
	t.Helper()
	l, err := parseLevel(level)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func captureStderr(t *testing.T, build func() *zap.Logger) (*zap.Logger, func() []byte) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	stderr := os.Stderr
	os.Stderr = f
	logger := build()
	os.Stderr = stderr

	return logger, func() []byte {
		t.Helper()
		out, err := os.ReadFile(f.Name())
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
}

func jsonLines(t *testing.T, out []byte) []map[string]any {
	t.Helper()
	var lines []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("invalid JSON log line %q: %v", scanner.Text(), err)
		}
		lines = append(lines, line)
	}
	return lines
}

func newTestLogger(t *testing.T, level string) (*zap.Logger, *logRecorder, func() []map[string]any) {
	t.Helper()
	recorder := &logRecorder{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(recorder))
	t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })

	l := mustParseLevel(t, level)
	logger, out := captureStderr(t, func() *zap.Logger { return initZap(l, lp) })
	return logger, recorder, func() []map[string]any { return jsonLines(t, out()) }
}

func TestInitZapLevel(t *testing.T) {
	tests := []struct {
		level string
		want  []string
	}{
		{level: "debug", want: []string{"debug", "info", "error"}},
		{level: "info", want: []string{"info", "error"}},
		{level: "error", want: []string{"error"}},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			logger, recorder, lines := newTestLogger(t, tt.level)
			logger.Debug("debug")
			logger.Info("info")
			logger.Error("error")

			var stderr, exported []string
			for _, line := range lines() {
				msg, _ := line["msg"].(string)
				stderr = append(stderr, msg)
			}
			for _, r := range recorder.get() {
				exported = append(exported, r.Body().AsString())
			}
			if !slices.Equal(stderr, tt.want) {
				t.Errorf("stderr: got %v, want %v", stderr, tt.want)
			}
			if !slices.Equal(exported, tt.want) {
				t.Errorf("exported: got %v, want %v", exported, tt.want)
			}
		})
	}
}

func TestInitZapSampling(t *testing.T) {
	logger, recorder, lines := newTestLogger(t, "info")
	for i := range 200 {
		logger.Info("sampled", zap.Int("i", i))
	}

	records := lines()
	stderr := make([]int64, 0, len(records))
	var exported []int64
	for _, line := range records {
		n, _ := line["i"].(float64)
		stderr = append(stderr, int64(n))
	}
	for _, r := range recorder.get() {
		r.WalkAttributes(func(kv attribute.KeyValue) bool {
			if kv.Key == "i" {
				exported = append(exported, kv.Value.AsInt64())
			}
			return true
		})
	}
	want := make([]int64, 0, 101)
	for i := range int64(100) {
		want = append(want, i)
	}
	want = append(want, 199)
	if !slices.Equal(stderr, want) {
		t.Errorf("stderr: got %v, want %v", stderr, want)
	}
	if !slices.Equal(exported, want) {
		t.Errorf("exported: got %v, want %v", exported, want)
	}
}

type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

func (fixedClock) NewTicker(d time.Duration) *time.Ticker { return time.NewTicker(d) }

func TestInitZapFormat(t *testing.T) {
	logger, out := captureStderr(t, func() *zap.Logger { return initZap(mustParseLevel(t, "info"), nil) })
	logger = logger.WithOptions(zap.WithClock(fixedClock(time.Date(2026, 11, 26, 10, 0, 0, 123e6, time.UTC))))

	logger.Info("hello", zap.String("key", "value"), zap.Duration("took", 1500*time.Millisecond), zap.Error(errors.New("boom")))

	want := `{"level":"info","ts":"2026-11-26T10:00:00.123Z","caller":"podinfo/main_test.go:L","msg":"hello","key":"value","took":1.5,"error":"boom"}` + "\n"
	if got := regexp.MustCompile(`main_test\.go:\d+`).ReplaceAllString(string(out()), "main_test.go:L"); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestWithTelemetryFatal(t *testing.T) {
	var mu sync.Mutex
	var exported []string
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req collogspb.ExportLogsServiceRequest
		if r.URL.Path == "/v1/logs" && proto.Unmarshal(body, &req) == nil {
			mu.Lock()
			for _, rl := range req.GetResourceLogs() {
				for _, sl := range rl.GetScopeLogs() {
					for _, lr := range sl.GetLogRecords() {
						exported = append(exported, lr.GetBody().GetStringValue())
					}
				}
			}
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	t.Cleanup(collector.Close)
	for _, k := range []string{
		"OTEL_SDK_DISABLED", "OTEL_LOGS_EXPORTER", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "OTEL_EXPORTER_OTLP_LOGS_PROTOCOL",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	eh, p, lp := otel.GetErrorHandler(), otel.GetTextMapPropagator(), otel.GetLoggerProvider()
	t.Cleanup(func() {
		otel.SetErrorHandler(eh)
		otel.SetTextMapPropagator(p)
		otel.SetLoggerProvider(lp)
	})

	tel, err := telemetry.Setup(context.Background(), "", zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tel.Shutdown(context.Background()) })
	var codes []int
	logger, out := captureStderr(t, func() *zap.Logger { //nolint:staticcheck // logger.Fatal returns, the fatal hook does not exit
		return withTelemetry(mustParseLevel(t, "info"), tel, time.Minute, func(code int) { codes = append(codes, code) })
	})

	logger.Fatal("fatal")

	if !slices.Equal(codes, []int{1}) {
		t.Errorf("exit codes: got %v, want [1]", codes)
	}
	mu.Lock()
	if !slices.Equal(exported, []string{"fatal"}) {
		t.Errorf("exported: got %v, want [fatal]", exported)
	}
	mu.Unlock()
	if tel.LoggerProvider().Logger("test").Enabled(context.Background(), log.EnabledParameters{}) {
		t.Error("logger provider not shut down")
	}
	if lines := jsonLines(t, out()); len(lines) != 1 || lines[0]["level"] != "fatal" {
		t.Errorf("stderr: got %v, want the fatal record", lines)
	}
}
func TestInitZapTraceContext(t *testing.T) {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	spanCtx, span := tp.Tracer("test").Start(context.Background(), "test")
	defer span.End()

	remoteCtx := propagation.TraceContext{}.Extract(context.Background(), propagation.HeaderCarrier{
		"Traceparent": []string{"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
	})
	remoteCtx, remoteSpan := noop.NewTracerProvider().Tracer("test").Start(remoteCtx, "test")
	defer remoteSpan.End()
	field := zap.String("key", "value")

	tests := []struct {
		name string
		log  func(*zap.Logger)
		want trace.SpanContext
	}{
		{
			name: "no context",
			log:  func(l *zap.Logger) { l.Info("msg", field) },
		},
		{
			name: "context without span",
			log:  func(l *zap.Logger) { l.Info("msg", field, telemetry.Context(context.Background())) },
		},
		{
			name: "recording span",
			log:  func(l *zap.Logger) { l.Info("msg", field, telemetry.Context(spanCtx)) },
			want: span.SpanContext(),
		},
		{
			name: "recording span with logger.With",
			log:  func(l *zap.Logger) { l.With(telemetry.Context(spanCtx)).Info("msg", field) },
			want: span.SpanContext(),
		},
		{
			name: "remote span without tracer provider",
			log:  func(l *zap.Logger) { l.Info("msg", field, telemetry.Context(remoteCtx)) },
			want: trace.SpanContextFromContext(remoteCtx),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, recorder, lines := newTestLogger(t, "info")
			tt.log(logger)

			records := recorder.get()
			if len(records) != 1 {
				t.Fatalf("got %d exported records, want 1", len(records))
			}
			r := records[0]
			if r.TraceID() != tt.want.TraceID() || r.SpanID() != tt.want.SpanID() || r.TraceFlags() != tt.want.TraceFlags() {
				t.Errorf("exported span context: got %s %s %s, want %s %s %s",
					r.TraceID(), r.SpanID(), r.TraceFlags(), tt.want.TraceID(), tt.want.SpanID(), tt.want.TraceFlags())
			}
			var attrs []string
			r.WalkAttributes(func(kv attribute.KeyValue) bool {
				attrs = append(attrs, string(kv.Key))
				return true
			})
			if !slices.Contains(attrs, "key") || slices.ContainsFunc(attrs, func(k string) bool {
				return k == "trace_id" || k == "span_id" || k == "context"
			}) {
				t.Errorf("exported attributes: got %v", attrs)
			}

			got := lines()
			if len(got) != 1 {
				t.Fatalf("got %d stderr lines, want 1", len(got))
			}
			line := got[0]
			wantKeys := []string{"caller", "key", "level", "msg", "ts"}
			if tt.want.IsValid() {
				wantKeys = append(wantKeys, "span_id", "trace_id")
				if line["trace_id"] != tt.want.TraceID().String() || line["span_id"] != tt.want.SpanID().String() {
					t.Errorf("stderr trace_id %v span_id %v, want %s %s", line["trace_id"], line["span_id"], tt.want.TraceID(), tt.want.SpanID())
				}
			}
			if keys := slices.Sorted(maps.Keys(line)); !slices.Equal(keys, slices.Sorted(slices.Values(wantKeys))) {
				t.Errorf("stderr keys: got %v, want %v", keys, wantKeys)
			}
		})
	}
}
