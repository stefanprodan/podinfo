# k8s-podinfo

Podinfo is a tiny web application made with Go 
that showcases best practices of running microservices in Kubernetes.

Specifications:

* Multi-arch build and release automation (TravisCI)
* Multi-platform Docker image (amd64/arm/arm64/ppc64le/s390x)
* Health checks (readiness and liveness)
* Graceful shutdown
* Standard multi-level logging (golang/glog)
* Prometheus instrumentation

Web API:

* `GET /` prints runtime information, environment variables, labels and annotations
* `GET /metrics` http requests duration and Go runtime metrics
* `GET /healthz` liveness probe
* `GET /readyz` readiness probe
* `POST /readyz/enable` enables readiness probe
* `POST /readyz/disable` disables readiness probe
* `GET /panic` crashes the process with exit code 255


### Instrumentation

Prometheus query examples of key metrics to measure and alert upon:

**Request Rate** - the number of requests per second by instance

```
sum(irate(http_requests_count{job=~".*podinfo"}[1m])) by (instance)
```

**Request Errors** - the number of failed requests per second by URL path

```
sum(irate(http_requests_count{job=~".*podinfo", status=~"5.."}[1m])) by (path)
```

**Request Duration** - average duration of each request over 10 minutes

```
sum(rate(http_requests_sum{job=~".*podinfo"}[10m])) / 
sum(rate(http_requests_count{job=~".*podinfo"}[10m]))
```

**Request Latency** - 99th percentile request latency over 10 minutes

```
histogram_quantile(0.99, sum(rate(http_requests_bucket{job=~".*podinfo"}[10m])) by (le))
```

**Goroutines Rate** - the number of running goroutines over 10 minutes

```
sum(irate(go_goroutines{job=~".*podinfo"}[10m]))
```

**Memory Usage** - the average number of bytes in use by instance

```
avg(go_memstats_alloc_bytes{job=~".*podinfo"}) by (instance)
```

**GC Duration** -  average duration of GC invocations over 10 minutes

```
sum(rate(go_gc_duration_seconds_sum{job=~".*podinfo"}[10m])) / 
sum(rate(go_gc_duration_seconds_count{job=~".*podinfo"}[10m]))
```

