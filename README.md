# k8s-podinfo

Podinfo is a tiny web application made with Go 
that showcases best practices of running microservices in Kubernetes.

Specifications:

* Multi-arch build and release automation (Make/TravisCI)
* Multi-platform Docker image (amd64/arm/arm64/ppc64le/s390x)
* Health checks (readiness and liveness)
* Graceful shutdown on interrupt signals
* Prometheus instrumentation (RED metrics)
* Dependency management with golang/dep
* Multi-level logging with golang/glog
* Error handling with pkg/errors
* Helm chart

Web API:

* `GET /` prints runtime information, environment variables, labels and annotations
* `GET /version` prints podinfo version and git commit hash 
* `GET /metrics` http requests duration and Go runtime metrics
* `GET /healthz` used by Kubernetes liveness probe
* `GET /readyz` used by Kubernetes readiness probe
* `POST /readyz/enable` signals the Kubernetes LB that this instance is ready to receive traffic
* `POST /readyz/disable` signals the Kubernetes LB to stop sending requests to this instance
* `GET /panic` crashes the process with exit code 255
* `POST /echo` echos the posted content, logs the SHA1 hash of the content
* `POST /job` long running job, json body: `{"wait":2}` 
* `POST /write` writes the posted content to disk at /data/hash and returns the SHA1 hash of the content
* `POST /read` receives a SHA1 hash and returns the content of the file /data/hash if exists
* `POST /backend` forwards the call to the backend service on `http://backend-podinfo:9898/echo`

### Guides

* [Deploy and upgrade with Helm](docs/1-deploy.md)
* [Horizontal Pod Auto-scaling](docs/2-autoscaling.md)
* [Monitoring and alerting with Prometheus](docs/3-monitoring.md)

