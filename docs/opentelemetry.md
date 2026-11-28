# Podinfo OpenTelemetry

Podinfo exports traces, metrics and logs over OTLP, and is configured with the standard
[OTEL_*](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/) environment variables.
Telemetry is off until an endpoint is set:

```bash
docker run -dp 9898:9898 \
-e OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318 \
-e OTEL_SERVICE_NAME=podinfo \
stefanprodan/podinfo
```

| Variable                      | Description                                                                         |
|-------------------------------|-------------------------------------------------------------------------------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Collector address, enables the three signals                                        |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` on port 4318 (default) or `grpc` on port 4317                       |
| `OTEL_SERVICE_NAME`           | Service name, defaults to `podinfo`                                                 |
| `OTEL_METRICS_EXPORTER`       | Set to `none` to keep traces and logs only                                          |
| `OTEL_PROPAGATORS`            | Defaults to `tracecontext,baggage`; add `b3`, `jaeger` or `ottrace` for those headers |
| `OTEL_SDK_DISABLED`           | Set to `true` to turn telemetry off                                                 |

HTTP and gRPC requests, the calls to backends and the cache commands are traced, and the trace context
is propagated from frontend to backend. Logs follow `--level` and carry the `trace_id` and `span_id`;
request logs are written at debug level. The Prometheus `/metrics` endpoint is always on, and its
histograms carry exemplars when scraped as OpenMetrics.

The [otel](../otel/README.md) directory runs two podinfo instances with Grafana otel-lgtm,
with traces, logs and metrics correlated in Grafana.
