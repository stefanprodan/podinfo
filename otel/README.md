# Tracing & Logging Demo

The directory contains sample [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector)
and [Jaeger](https://www.jaegertracing.io) / [Loki](https://grafana.com/oss/loki/) configurations for a tracing and logging demo.

## Configuration

The provided [docker-compose.yaml](docker-compose.yaml) sets up 6 containers:

1. PodInfo Frontend on port 9898
2. PodInfo Backend on port 9899
3. OpenTelemetry Collector listening on port 4317 for GRPC
4. Jaeger all-in-one with UI on port 16686
5. Loki on port 3100
6. Grafana on port 3000

## How does it work?

The frontend pod is configured to call the backend pod. Both podinfo pods send traces
and logs to the collector at port 4317 using OTLP gRPC.

The collector forwards:
- **Traces** to Jaeger via OTLP gRPC on port 4317
- **Logs** to Loki via OTLP HTTP on port 3100

Jaeger exposes its UI on port `16686`. Grafana exposes its UI on port `3000` and is
pre-configured with both Jaeger and Loki as datasources.

## Running it locally

1. Start all the containers
```shell
make run
```
2. Send some sample requests
```shell
curl -v http://localhost:9898/status/200
curl -X POST -v http://localhost:9898/api/echo
```
3. Visit `http://localhost:16686/` to see traces in Jaeger
4. Visit `http://localhost:3000/` to explore logs in Grafana (Explore → Loki) and traces (Explore → Jaeger)
5. Stop all the containers
```shell
make stop
```
