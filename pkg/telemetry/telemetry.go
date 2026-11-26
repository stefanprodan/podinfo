package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stefanprodan/podinfo/pkg/version"
)

const (
	instrumentationName = "github.com/stefanprodan/podinfo/pkg/api"
	defaultServiceName  = "podinfo"
)

// Telemetry holds the OpenTelemetry providers configured by Setup.
type Telemetry struct {
	logger         *zap.Logger
	loggerProvider *sdklog.LoggerProvider
	shutdowns      []func(context.Context) error
	shutdownOnce   sync.Once
}

// Setup configures OpenTelemetry from the OTEL_* environment variables; serviceName applies when OTEL_SERVICE_NAME is not set.
func Setup(ctx context.Context, serviceName string, logger *zap.Logger) (*Telemetry, error) {
	t := &Telemetry{logger: logger}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")), "true") {
		return t, nil
	}

	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Warn("OpenTelemetry error", zap.Error(err))
	}))
	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())

	if !signalEnabled("TRACES") && !signalEnabled("LOGS") && !signalEnabled("METRICS") {
		return t, nil
	}

	res, err := newResource(ctx, serviceName)
	if err != nil {
		logger.Warn("detecting OpenTelemetry resource", zap.Error(err))
	}

	tp, err := newTracerProvider(ctx, res)
	if err != nil {
		return t, err
	}
	if tp != nil {
		t.shutdowns = append(t.shutdowns, tp.Shutdown)
	}

	lp, err := newLoggerProvider(ctx, res)
	if err != nil {
		_ = t.Shutdown(context.WithoutCancel(ctx))
		return t, err
	}
	if lp != nil {
		t.shutdowns = append(t.shutdowns, lp.Shutdown)
	}

	mp, err := newMeterProvider(ctx, res)
	if err != nil {
		_ = t.Shutdown(context.WithoutCancel(ctx))
		return t, err
	}
	if mp != nil {
		t.shutdowns = append(t.shutdowns, mp.Shutdown)
	}

	if tp != nil {
		otel.SetTracerProvider(tp)
	}
	if lp != nil {
		otel.SetLoggerProvider(lp)
		t.loggerProvider = lp
	}
	if mp != nil {
		otel.SetMeterProvider(mp)
	}

	return t, nil
}

// LoggerProvider returns the OTLP logger provider, or nil when log export is disabled.
func (t *Telemetry) LoggerProvider() *sdklog.LoggerProvider {
	return t.loggerProvider
}

// Shutdown flushes and stops the providers. Calls after the first are no-ops.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var err error
	t.shutdownOnce.Do(func() {
		errs := make([]error, len(t.shutdowns))
		var wg sync.WaitGroup
		for i, shutdown := range t.shutdowns {
			wg.Go(func() {
				errs[i] = shutdown(ctx)
			})
		}
		wg.Wait()
		err = errors.Join(errs...)
	})
	return err
}

// ExitHook returns a zap hook for fatal entries that shuts down telemetry within timeout and then calls exit(1).
func (t *Telemetry) ExitHook(timeout time.Duration, exit func(int)) zapcore.CheckWriteHook {
	return exitHook{telemetry: t, timeout: timeout, exit: exit}
}

type exitHook struct {
	telemetry *Telemetry
	timeout   time.Duration
	exit      func(int)
}

func (h exitHook) OnWrite(*zapcore.CheckedEntry, []zapcore.Field) {
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	if err := h.telemetry.Shutdown(ctx); err != nil {
		h.telemetry.logger.Warn("OpenTelemetry shutdown failed", zap.Error(err))
	}
	cancel()
	h.exit(1)
}

// Tracer returns the podinfo tracer from the global tracer provider.
func Tracer() trace.Tracer {
	return otel.Tracer(
		instrumentationName,
		trace.WithInstrumentationVersion(version.VERSION),
		trace.WithSchemaURL(semconv.SchemaURL),
	)
}

// Observe records v on o, with the trace_id and span_id of the recording, sampled span in ctx as exemplar.
func Observe(ctx context.Context, o prometheus.Observer, v float64) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if eo, ok := o.(prometheus.ExemplarObserver); ok && span.IsRecording() && sc.IsSampled() {
		eo.ObserveWithExemplar(v, prometheus.Labels{
			"trace_id": sc.TraceID().String(),
			"span_id":  sc.SpanID().String(),
		})
		return
	}
	o.Observe(v)
}

func signalEnabled(signal string) bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" ||
		os.Getenv("OTEL_EXPORTER_OTLP_"+signal+"_ENDPOINT") != "" ||
		os.Getenv("OTEL_"+signal+"_EXPORTER") != ""
}

func newResource(ctx context.Context, serviceName string) (*resource.Resource, error) {
	opts := []resource.Option{
		resource.WithService(),
		resource.WithAttributes(
			semconv.ServiceName(defaultServiceName),
			semconv.ServiceVersion(version.VERSION),
		),
		resource.WithTelemetrySDK(),
		resource.WithProcessPID(),
		resource.WithProcessExecutableName(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithProcessRuntimeDescription(),
		resource.WithHost(),
		resource.WithOSType(),
		resource.WithContainerID(),
		resource.WithFromEnv(),
	}
	if serviceName != "" && strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")) == "" {
		opts = append(opts, resource.WithAttributes(semconv.ServiceName(serviceName)))
	}
	return resource.New(ctx, opts...)
}

func newTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	if !signalEnabled("TRACES") {
		return nil, nil
	}
	exporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating span exporter: %w", err)
	}
	if autoexport.IsNoneSpanExporter(exporter) {
		return nil, nil
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	), nil
}

func newLoggerProvider(ctx context.Context, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	if !signalEnabled("LOGS") {
		return nil, nil
	}
	exporter, err := autoexport.NewLogExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating log exporter: %w", err)
	}
	if autoexport.IsNoneLogExporter(exporter) {
		return nil, nil
	}
	return sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	), nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	if !signalEnabled("METRICS") {
		return nil, nil
	}
	reader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating metric reader: %w", err)
	}
	if autoexport.IsNoneMetricReader(reader) {
		return nil, nil
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)
	if err := runtime.Start(runtime.WithMeterProvider(mp)); err != nil {
		_ = mp.Shutdown(context.WithoutCancel(ctx))
		return nil, fmt.Errorf("starting runtime metrics: %w", err)
	}
	return mp, nil
}
