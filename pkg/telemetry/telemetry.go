package telemetry

import (
	"context"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/contrib/propagators/ot"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/version"
)

const instrumentationName = "github.com/stefanprodan/podinfo/pkg/api"

// NewResource returns the SDK default resource merged with the podinfo service name and version.
func NewResource(serviceName string) (*resource.Resource, error) {
	return resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(version.VERSION),
	))
}

// NewTracerProvider sets a global OTLP tracer provider, or returns nil and leaves the global one untouched.
func NewTracerProvider(ctx context.Context, serviceName string, logger *zap.Logger) *sdktrace.TracerProvider {
	if serviceName == "" {
		return nil
	}

	res, err := NewResource(serviceName)
	if err != nil {
		logger.Error("creating OpenTelemetry resource, tracing disabled", zap.Error(err))
		return nil
	}

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		logger.Error("creating OTLP trace exporter, tracing disabled", zap.Error(err))
		return nil
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
		b3.New(),
		&jaeger.Jaeger{},
		&ot.OT{},
	))

	return tp
}

// Tracer returns the podinfo tracer from the global tracer provider.
func Tracer() trace.Tracer {
	return otel.Tracer(
		instrumentationName,
		trace.WithInstrumentationVersion(version.VERSION),
		trace.WithSchemaURL(semconv.SchemaURL),
	)
}
