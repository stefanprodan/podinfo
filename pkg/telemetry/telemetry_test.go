package telemetry

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/version"
)

func TestMain(m *testing.M) {
	os.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.name=from-env,deployment.environment.name=test")
	os.Exit(m.Run())
}

func resetGlobals(t *testing.T) {
	tp := otel.GetTracerProvider()
	p := otel.GetTextMapPropagator()
	otel.SetTracerProvider(noop.NewTracerProvider())
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	t.Cleanup(func() {
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(p)
	})
}

func TestNewTracerProviderDisabled(t *testing.T) {
	resetGlobals(t)
	global := otel.GetTracerProvider()

	if tp := NewTracerProvider(context.Background(), "", zap.NewNop()); tp != nil {
		t.Fatalf("expected nil tracer provider, got %T", tp)
	}

	if otel.GetTracerProvider() != global {
		t.Fatal("expected the global tracer provider to be left untouched")
	}

	_, span := Tracer().Start(context.Background(), "test")
	if span.SpanContext().IsValid() {
		t.Fatal("expected a noop span")
	}
}

func TestNewResource(t *testing.T) {
	res, err := NewResource("podinfo-test")
	if err != nil {
		t.Fatal(err)
	}

	want := map[attribute.Key]string{
		"service.name":                "podinfo-test",
		"service.version":             version.VERSION,
		"deployment.environment.name": "test",
		"telemetry.sdk.language":      "go",
	}
	set := res.Set()
	for k, v := range want {
		got, ok := set.Value(k)
		if !ok || got.AsString() != v {
			t.Errorf("%s: got %q, want %q", k, got.AsString(), v)
		}
	}
}

func TestNewTracerProvider(t *testing.T) {
	resetGlobals(t)
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://127.0.0.1:4317")

	tp := NewTracerProvider(context.Background(), "podinfo-test", zap.NewNop())
	if tp == nil {
		t.Fatal("expected a tracer provider")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = tp.Shutdown(ctx)
	})

	if otel.GetTracerProvider() != tp {
		t.Fatal("expected the global tracer provider to be set")
	}

	fields := otel.GetTextMapPropagator().Fields()
	for _, f := range []string{"traceparent", "baggage", "b3", "uber-trace-id", "ot-tracer-traceid"} {
		if !slices.Contains(fields, f) {
			t.Errorf("propagator fields %v missing %q", fields, f)
		}
	}

	_, span := Tracer().Start(context.Background(), "test")
	defer span.End()
	if !span.SpanContext().IsValid() {
		t.Fatal("expected a recording span")
	}
}
