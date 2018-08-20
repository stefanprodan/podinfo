# k8s-podinfo

Podinfo is a tiny web application made with Go 
that showcases best practices of running microservices in Kubernetes.

Specifications:

* Release automation (Make/TravisCI/CircleCI/Quay.io/Google Cloud Container Builder/Skaffold/Weave Flux)
* Multi-platform Docker image (amd64/arm/arm64/ppc64le/s390x)
* Health checks (readiness and liveness)
* Graceful shutdown on interrupt signals
* Watches for secrets and configmaps changes and updates the in-memory cache
* Prometheus instrumentation (RED metrics)
* Dependency management with golang/dep
* Structured logging with zap
* Tracing with Istio and Jaeger
* 12-factor app with viper
* Helm chart

Web API:

* `GET /` prints runtime information
* `GET /version` prints podinfo version and git commit hash 
* `GET /metrics` return HTTP requests duration and Go runtime metrics
* `GET /healthz` used by Kubernetes liveness probe
* `GET /readyz` used by Kubernetes readiness probe
* `POST /readyz/enable` signals the Kubernetes LB that this instance is ready to receive traffic
* `POST /readyz/disable` signals the Kubernetes LB to stop sending requests to this instance
* `GET /status/{code}` returns the status code
* `GET /panic` crashes the process with exit code 255
* `POST /echo` forwards the call to the backend service and echos the posted content 
* `GET /headers` returns a JSON with the request HTTP headers
* `GET /delay/{seconds}` waits for the specified period
* `GET /configs` returns a JSON with configmaps and/or secrets mounted in the `config` volume
* `POST /write` writes the posted content to disk at /data/hash and returns the SHA1 hash of the content
* `GET /read/{hash}` returns the content of the file /data/hash if exists

### Guides

* [Deploy and upgrade with Helm](docs/1-deploy.md)
* [Horizontal Pod Auto-scaling](docs/2-autoscaling.md)
* [Monitoring and alerting with Prometheus](docs/3-monitoring.md)
* [StatefulSets with local persistent volumes](docs/4-statefulsets.md)
* [Expose Kubernetes services over HTTPS with Ngrok](docs/6-ngrok.md)
* [A/B Testing with Ambassador API Gateway](docs/5-canary.md)
* [Canary Deployments with Istio](docs/7-istio.md)
