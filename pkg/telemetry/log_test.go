package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func spanContext(b byte) context.Context {
	return trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{b},
		SpanID:     trace.SpanID{b},
		TraceFlags: trace.FlagsSampled,
	}))
}

func TestTraceIDCore(t *testing.T) {
	first, second := spanContext(1), spanContext(2)
	tests := []struct {
		name string
		log  func(*zap.Logger)
		want context.Context //nolint:containedctx // test case expectation
	}{
		{name: "none", log: func(l *zap.Logger) { l.Info("msg") }},
		{name: "invalid span context", log: func(l *zap.Logger) { l.Info("msg", Context(context.Background())) }},
		{name: "nil context", log: func(l *zap.Logger) { l.Info("msg", Context(nil)) }}, //nolint:staticcheck // covers a nil context field
		{name: "field", log: func(l *zap.Logger) { l.Info("msg", Context(first)) }, want: first},
		{name: "last field wins", log: func(l *zap.Logger) { l.Info("msg", Context(first), Context(second)) }, want: second},
		{name: "with", log: func(l *zap.Logger) { l.With(Context(first)).Info("msg") }, want: first},
		{name: "field over with", log: func(l *zap.Logger) { l.With(Context(first)).Info("msg", Context(second)) }, want: second},
		{name: "later with wins", log: func(l *zap.Logger) { l.With(Context(first)).With(Context(second)).Info("msg") }, want: second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.log(zap.New(NewTraceIDCore(zapcore.NewCore(
				zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "msg"}), zapcore.AddSync(&buf), zapcore.InfoLevel))))

			want := map[string]any{"msg": "msg"}
			if tt.want != nil {
				sc := trace.SpanContextFromContext(tt.want)
				want["trace_id"] = sc.TraceID().String()
				want["span_id"] = sc.SpanID().String()
			}
			var got map[string]any
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON %q: %v", buf.String(), err)
			}
			if len(got) != len(want) || got["trace_id"] != want["trace_id"] || got["span_id"] != want["span_id"] {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func TestTraceIDCoreKeepsFields(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	logger := zap.New(NewTraceIDCore(zapcore.NewCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "msg"}),
		zapcore.AddSync(writerFunc(func(p []byte) {
			mu.Lock()
			lines = append(lines, string(p))
			mu.Unlock()
		})),
		zapcore.InfoLevel,
	))).With(Context(spanContext(1)))

	fields := make([]zap.Field, 1, 4)
	fields[0] = zap.String("key", "value")
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { logger.Info("msg", fields...) })
	}
	wg.Wait()

	if len(lines) != 10 {
		t.Errorf("got %d lines, want 10", len(lines))
	}
	if fields[:2][1] != (zap.Field{}) {
		t.Errorf("caller fields modified: %v", fields[:2])
	}
	want := `{"msg":"msg","key":"value","trace_id":"01000000000000000000000000000000","span_id":"0100000000000000"}` + "\n"
	for _, line := range lines {
		if line != want {
			t.Errorf("got %s, want %s", line, want)
		}
	}
}

type writerFunc func([]byte)

func (f writerFunc) Write(p []byte) (int, error) {
	f(p)
	return len(p), nil
}

func TestTraceIDCoreAllocs(t *testing.T) {
	if raceEnabled {
		t.Skip("sync.Pool drops a random quarter of the objects put back under the race detector, so zap allocates a random number of times per record")
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "msg"}), zapcore.AddSync(io.Discard), zapcore.InfoLevel)
	plain, wrapped := zap.New(core), zap.New(NewTraceIDCore(core))

	want := testing.AllocsPerRun(100, func() { plain.Info("msg") })
	if got := testing.AllocsPerRun(100, func() { wrapped.Info("msg") }); got != want {
		t.Errorf("got %v allocs per record, want %v", got, want)
	}
}
