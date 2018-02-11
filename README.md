# k8s-podinfo

Podinfo is a tiny web application made with Go 
that showcases best practices of running microservices in Kubernetes.

Specifications:

* Multi-arch build and release automation (Make/TravisCI)
* Multi-platform Docker image (amd64/arm/arm64/ppc64le/s390x)
* Health checks (readiness and liveness)
* Graceful shutdown on interrupt signals
* Prometheus instrumentation
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
* `POST /backend` forwards the call to the backend service on `http://backend-podinfo:9898/echo`

### Deployment

Install Helm and deploy Tiller on your Kubernetes cluster:

```bash
# install Helm CLI
brew install kubernetes-helm

# create a service account for Tiller
kubectl -n kube-system create sa tiller

# create a cluster role binding for Tiller
kubectl create clusterrolebinding tiller-cluster-rule \
    --clusterrole=cluster-admin \
    --serviceaccount=kube-system:tiller 

# deploy Tiller in kube-system namespace
helm init --skip-refresh --upgrade --service-account tiller
```

Install the frontend release exposed via a NodePort service:

```bash
helm upgrade --install --wait frontend \
    --set service.type=NodePort \
    --set service.nodePort=31198 \
    ./chart/stable/podinfo
```

Check if podinfo service is accessible from within the cluster:

```bash
helm test --cleanup frontend
```

Set CPU/memory requests and limits:

```bash
helm upgrade --install --wait frontend \
    --set resources.requests.cpu=10m \
    --set resources.limits.cpu=100m \
    --set resources.requests.memory=16Mi \
    --set resources.limits.memory=128Mi \
    ./chart/stable/podinfo
```

Install the backend release with horizontal pod autoscaling (HPA) based on CPU average usage and memory consumption:

```bash
helm upgrade --install --wait backend \
    --set hpa.enabled=true \
    --set hpa.maxReplicas=10 \
    --set hpa.cpu=80 \
    --set hpa.memory=200Mi \
    ./chart/stable/podinfo
```

Update podinfo version:

```bash
helm upgrade frontend \
    --set image.tag=0.0.4 \
    ./chart/stable/podinfo
```

Rollback the last deploy:

```bash
helm rollback frontend
```

Delete the releases:

```bash
helm delete --purge frontend backend
```

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

