// Package telemetry sets up OpenTelemetry traces, metrics and logs for
// podinfo and ties them to its zap logs and Prometheus metrics.
//
// Setup reads the standard OTEL_* environment variables. Signals are
// exported when an OTLP endpoint or an explicit exporter is configured, and
// OTEL_SDK_DISABLED turns everything off. Shutdown flushes the providers,
// and ExitHook does so before a fatal log entry exits the process.
//
// NewTraceIDCore adds trace_id and span_id to JSON log lines logged with
// Context, and Observe records a Prometheus observation with the current
// trace as exemplar.
package telemetry
