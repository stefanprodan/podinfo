# OpenTelemetry demo

This directory runs two podinfo instances that send traces, metrics and logs
over OTLP to [Grafana otel-lgtm](https://github.com/grafana/docker-otel-lgtm),
a single container with an OpenTelemetry Collector, Tempo, Loki, Prometheus and Grafana.

## What runs

| Service    | Ports                          | Role                                                |
|------------|--------------------------------|-----------------------------------------------------|
| `frontend` | 9898 (HTTP), 9999 (gRPC)       | podinfo, forwards `POST /echo` to the backend       |
| `backend`  | not published                  | podinfo, answers the frontend                       |
| `lgtm`     | 3000 (Grafana)                 | receives OTLP on 4318 from both podinfo instances   |

podinfo is configured only with the standard environment variables:

```yaml
OTEL_SERVICE_NAME: frontend
OTEL_EXPORTER_OTLP_ENDPOINT: http://lgtm:4318
OTEL_METRIC_EXPORT_INTERVAL: "10000"
```

The protocol is the default `http/protobuf`. Metrics are exported every 10 seconds
instead of the default 60. Both instances run with `--level debug`, because the
log records written while serving a request are at debug level.

[grafana-datasources.yaml](grafana-datasources.yaml) replaces the Tempo datasource
that otel-lgtm provisions, to add links from a span to its HTTP and gRPC metrics.

## Run it

Start the stack. The command returns when Grafana is ready:

```shell
make run
```

Send an HTTP request that goes through the frontend to the backend, with a known trace ID:

```shell
curl -s -H 'traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01' \
  -d 'hello' http://localhost:9898/echo
```

Make a gRPC call to the frontend with [grpcurl](https://github.com/fullstorydev/grpcurl):

```shell
grpcurl -plaintext -H 'traceparent: 00-1af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01' \
  -d '{"body":"hello"}' localhost:9999 echo.v1.EchoService/Echo
```

Generate some traffic for the metrics:

```shell
for i in $(seq 100); do curl -s -d 'hello' http://localhost:9898/echo > /dev/null; done
```

Stop the stack and remove the containers, volumes and the podinfo images:

```shell
make clean
```

## Explore in Grafana

Open <http://localhost:3000> (anonymous admin access).

**Traces.** Go to Explore, pick Tempo, choose the TraceQL query type and paste
`0af7651916cd43dd8448eb211c80319c`. The trace has three spans: the frontend
`POST /echo` server span, its `POST` client span and the backend `POST /echo`
server span. Trace `1af7651916cd43dd8448eb211c80319c` holds one span, the gRPC
server span `echo.v1.EchoService/Echo`.
Both traces start with a parent span that only exists in the `traceparent` header,
so Tempo search lists them as `<root span not yet received>`.

**Metrics.** Open Dashboards, RED Metrics (classic histogram), and select the
`frontend` or `backend` job. In Explore with Prometheus, query:

- `http_server_request_duration_seconds_count` for HTTP requests
- `rpc_server_call_duration_seconds_count` for gRPC calls
- `go_memory_used_bytes` and `go_goroutine_count` for the Go runtime

Turn on Exemplars on a histogram query to jump from a data point to its trace.
In the trace view, the Links menu on a span opens its request rate, p95 latency
or gRPC call rate.

**Logs.** In Explore with Loki, query `{service_name="frontend"}` or
`{service_name="backend"}`. Records written while serving a request carry the
trace and span ID of the server span. In the trace view, Related logs shows the
frontend lines for the trace (`request started`, `payload received from backend`),
and the Links menu on the backend span opens the backend lines (`request started`,
`payload received`). The gRPC call logs `received message body from client`.
Expand a log line and click `Trace: <trace ID>` under Links to open the trace in Tempo.
