# podinfo

[![e2e](https://github.com/stefanprodan/podinfo/workflows/e2e/badge.svg)](https://github.com/stefanprodan/podinfo/blob/master/.github/workflows/e2e.yml)
[![test](https://github.com/stefanprodan/podinfo/workflows/test/badge.svg)](https://github.com/stefanprodan/podinfo/blob/master/.github/workflows/test.yml)
[![cve-scan](https://github.com/stefanprodan/podinfo/workflows/cve-scan/badge.svg)](https://github.com/stefanprodan/podinfo/blob/master/.github/workflows/cve-scan.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/stefanprodan/podinfo)](https://hub.docker.com/r/stefanprodan/podinfo)
[![slsa](https://slsa.dev/images/gh-badge-level2.svg)](https://github.com/stefanprodan/podinfo/attestations)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/stefanprodan/podinfo)

Podinfo is a tiny web application made with Go that showcases best practices of running microservices in Kubernetes.
Podinfo is used by CNCF projects like [Flux](https://github.com/fluxcd/flux2) and [Flagger](https://github.com/fluxcd/flagger)
for end-to-end testing and workshops.

![podinfo-ui](https://raw.githubusercontent.com/stefanprodan/podinfo/gh-pages/screens/podinfo-ui-v7.png)

### Specifications

Features:

* Health checks (readiness and liveness)
* Graceful shutdown on interrupt signals
* File watcher for secrets and configmaps
* Instrumented with Prometheus and OpenTelemetry (traces, metrics and logs over OTLP)
* Structured logging with zap 
* 12-factor app with viper
* Fault injection (random errors and latency)
* TLS and SPIFFE mTLS on the HTTPS and gRPC ports, with X.509 SVIDs from the SPIFFE Workload API
* Redis and Valkey cache in standalone, Cluster and Sentinel mode, with TLS and mTLS
* Web UI embedded in the binary, with no external requests
* Swagger docs
* Timoni, Helm and Kustomize installers
* End-to-End testing with Kubernetes Kind and Helm
* Multi-arch container image with Docker buildx and GitHub Actions
* Container image signing with Sigstore cosign
* SBOMs and SLSA Provenance embedded in the container image
* CVE scanning with govulncheck

Web API:

* `GET /` serves the web UI to browsers and prints the runtime information to other clients
* `GET /api/info` returns the runtime information as JSON, polled by the web UI
* `GET /api/pod` returns the runtime information, the environment variables and the request headers of one pod, with secret values masked, polled by the web UI while its spec sheet is open
* `GET /version` prints podinfo version and git commit hash 
* `GET /metrics` return HTTP requests duration and Go runtime metrics
* `GET /healthz` used by Kubernetes liveness probe
* `GET /readyz` used by Kubernetes readiness probe
* `POST /readyz/enable` signals the Kubernetes LB that this instance is ready to receive traffic
* `POST /readyz/disable` signals the Kubernetes LB to stop sending requests to this instance
* `POST /fault_injection/enable` makes this instance respond with HTTP 500 to all application endpoints (probes, metrics, pprof and the `/fault_injection/*` control endpoints stay healthy) — useful for testing client-side circuit breakers / outlier detection against a single "sick" replica
* `POST /fault_injection/disable` restores normal responses
* `GET /fault_injection/status` returns the current fault injection state (`enabled` or `disabled`)
* `GET /status/{code}` returns the status code
* `GET /panic` crashes the process with exit code 255
* `POST /echo` forwards the call to the backend service and echos the posted content, without backends the content is logged at debug level
* `GET /env` returns the environment variables as a JSON array, with secret values masked
* `GET /headers` returns a JSON with the request HTTP headers, with credential values masked, plus `X-Spiffe-Id` for SPIFFE mTLS clients
* `GET /delay/{seconds}` waits for the specified period
* `POST /token` issues a JWT token valid for one minute `JWT=$(curl -sd 'anon' podinfo:9898/token | jq -r .token)`
* `GET /token/validate` validates the JWT token `curl -H "Authorization: Bearer $JWT" podinfo:9898/token/validate`
* `GET /configs` returns a JSON with configmaps and/or secrets mounted in the `config` volume
* `POST/PUT /cache/{key}` saves the posted content to Redis or Valkey
* `GET /cache/{key}` returns the content from Redis or Valkey if the key exists
* `DELETE /cache/{key}` deletes the key from Redis or Valkey if exists
* `POST /store` writes the posted content to disk at /data/hash and returns the SHA1 hash of the content
* `GET /store/{hash}` returns the content of the file /data/hash if exists
* `GET/HEAD /artifact` returns the files from `--artifact-path` (default `./kustomize`) as a tar.gz, with the sha256 digest in the `X-Artifact-Digest` header
* `GET /ws/echo` echos content via websockets `podcli ws ws://localhost:9898/ws/echo`
* `GET /chunked/{seconds}` uses `transfer-encoding` type `chunked` to give a partial response and then waits for the specified period
* `GET /swagger.json` returns the API Swagger docs, used for Linkerd service profiling and Gloo routes discovery

Errors are returned with their HTTP status code and a JSON body with the `code` and the `message`.

gRPC API:

* `/grpc.health.v1.Health/Check` health checking
* `/echo.v1.EchoService/Echo` echos the received content
* `/version.v1.VersionService/Version` returns podinfo version and Git commit hash
* `/delay.v1.DelayService/Delay` returns a successful response after the given seconds in the body of gRPC request
* `/env.v1.EnvService/Env` returns environment variables as a JSON array, with secret values masked
* `/header.v1.HeaderService/Header` returns the headers present in the gRPC request, with credential values masked. Any custom header can also be given as a part of request and that can be returned using this API
* `/info.v1.InfoService/Info` returns the runtime information
* `/pod.v1.PodService/Pod` returns the runtime information, the environment variables and the request metadata of one pod, with secret values masked
* `/panic.v1.PanicService/Panic` crashes the process with gRPC status code as '1 CANCELLED'
* `/status.v1.StatusService/Status` returns the gRPC Status code given in the request body
* `/token.v1.TokenService/TokenGenerate` issues a JWT token valid for one minute
* `/token.v1.TokenService/TokenValidate` validates the JWT token

Web UI:

The UI shows the version and the pod that answered, each version's share of the last 30 responses,
and when the API stopped answering. It is set up with flags:

| Flag           | Description                                                                              |
|----------------|------------------------------------------------------------------------------------------|
| `--ui-color`   | Background color, defaults to the blue `#1f4e8c`; `#17694f` is the green that goes with it |
| `--ui-message` | Greeting, defaults to `greetings from podinfo`                                           |
| `--ui-logo`    | Image URL, defaults to the embedded one                                                  |

To see the UI during a canary release without a cluster, run `make ui-demo` and open `http://localhost:9898`.

To access the Swagger UI open `<podinfo-host>/swagger/index.html` in a browser.

### Guides

* [Getting started with Timoni](https://timoni.sh/quickstart/)
* [Getting started with Flux](https://fluxcd.io/flux/get-started/)
* [Progressive Deliver with Flagger and Linkerd](https://docs.flagger.app/tutorials/linkerd-progressive-delivery)
* [Automated canary deployments with Kubernetes Gateway API](https://docs.flagger.app/tutorials/gatewayapi-progressive-delivery)

### Install

To install Podinfo on Kubernetes the minimum required version is **Kubernetes v1.23**
with Helm and Kustomize, and **Kubernetes v1.25** with Timoni.

#### Timoni

Install with [Timoni](https://timoni.sh):

```bash
timoni -n default apply podinfo oci://ghcr.io/stefanprodan/modules/podinfo
```

#### Helm

Install from github.io:

```bash
helm repo add podinfo https://stefanprodan.github.io/podinfo

helm upgrade --install --wait frontend \
--namespace test \
--set replicaCount=2 \
--set backend=http://backend-podinfo:9898/echo \
podinfo/podinfo

helm test frontend --namespace test

helm upgrade --install --wait backend \
--namespace test \
--set redis.enabled=true \
podinfo/podinfo
```

Install from ghcr.io:

```bash
helm upgrade --install --wait podinfo --namespace default \
oci://ghcr.io/stefanprodan/charts/podinfo
```

#### Kustomize

```bash
kubectl apply -k github.com/stefanprodan/podinfo//kustomize
```

#### Docker

```bash
docker run -dp 9898:9898 stefanprodan/podinfo
```

### OpenTelemetry

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

The [otel](otel/README.md) directory runs two podinfo instances with Grafana otel-lgtm,
with traces, logs and metrics correlated in Grafana.

### Cache

The `/cache/{key}` endpoints store values in Redis or Valkey:

```bash
podinfo --cache-server=redis://redis:6379
```

| Setting                                                     | Description                                                                                         |
|-------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|
| `--cache-server`                                            | `redis://[user:password@]host:port[/db]`, `rediss://` for TLS, `valkey://` and `valkeys://` as aliases |
| `--cache-cluster`                                           | Cluster mode, implied when the URL has more addresses as `?addr=host:port` parameters               |
| `--cache-sentinel-primary`                                  | Name of the primary monitored by Sentinel, the URL addresses are then the Sentinels                 |
| `--cache-ca-file`                                           | CA bundle that verifies the server certificate instead of the system roots                          |
| `--cache-cert-path`                                         | Directory with the `tls.crt` and `tls.key` client key pair for mTLS, reloaded on rotation           |
| `PODINFO_CACHE_USERNAME`, `PODINFO_CACHE_PASSWORD`          | Credentials, instead of putting them in the URL                                                     |
| `PODINFO_CACHE_SENTINEL_PASSWORD`                           | Password of the Sentinels                                                                           |

Without `--cache-server` the endpoints return HTTP 503. An invalid cache configuration is a startup error.

### SPIFFE mTLS

Podinfo can serve its HTTPS port with [SPIFFE](https://spiffe.io) mTLS. The server gets its X.509 SVID
from the SPIFFE Workload API (e.g. a [SPIRE](https://spiffe.io/docs/latest/spire-about/) agent),
rotates it automatically, and only accepts clients that present an SVID from the same trust bundle.

| Flag                       | Description                                                                                                                                 |
|----------------------------|---------------------------------------------------------------------------------------------------------------------------------------------|
| `--spiffe-endpoint-socket` | Workload API address, e.g. `unix:///spiffe-workload-api/spire-agent.sock`, requires `--secure-port`                                         |
| `--spiffe-authorized-ids`  | Client SPIFFE IDs allowed to connect, when empty any ID in podinfo's trust domain is allowed                                                |
| `--spiffe-backend-mtls`    | Call the `https` backends with SPIFFE mTLS, requires `--spiffe-endpoint-socket`                                                             |
| `--spiffe-backend-ids`     | Server SPIFFE IDs accepted from `https` backends, when empty any ID in podinfo's trust domain is accepted, requires `--spiffe-backend-mtls` |

Podinfo starts the HTTP port and passes the liveness probe right away, and keeps retrying the Workload API
until the SVID is issued. The readiness probe fails until the HTTPS port is serving with the SVID.
The request log includes the `spiffe_id` of the client at debug level.

With `--grpc-tls`, the gRPC port is served with the same SVID and authorized IDs, and the gRPC health
of the `--grpc-service-name` service is `NOT_SERVING` until the SVID is issued.
Without SPIFFE, `--grpc-tls` serves the gRPC port with the `--cert-path` certificate.

With `--spiffe-backend-mtls`, `/api/echo` calls the `https://` backends set with `--backend-url` over SPIFFE mTLS,
presenting the same SVID and verifying the backend against `--spiffe-backend-ids`.
All `https://` backends must then be SPIFFE workloads, calls to `http://` backends are unchanged.
Without the flag, backends are called with the default TLS settings. To chain two podinfo instances,
point the frontend at the backend's HTTPS port and authorize the frontend's ID on the backend:

```shell
# backend
podinfo --secure-port=9899 --spiffe-endpoint-socket=unix:///spiffe-workload-api/spire-agent.sock \
  --spiffe-authorized-ids=spiffe://example.org/ns/demo/sa/frontend

# frontend
podinfo --secure-port=9899 --spiffe-endpoint-socket=unix:///spiffe-workload-api/spire-agent.sock \
  --backend-url=https://backend:9899/api/echo \
  --spiffe-backend-mtls \
  --spiffe-backend-ids=spiffe://example.org/ns/demo/sa/backend
```

On a cluster with SPIRE and the [SPIFFE CSI driver](https://github.com/spiffe/spiffe-csi),
register the podinfo pods with the SPIRE controller manager:

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  name: podinfo
spec:
  # the SPIRE controller manager class, for the SPIRE Helm chart: <namespace>-<release>
  className: spire-system-spire
  spiffeIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
  podSelector: {}
  namespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: podinfo
```

Install with Helm:

```bash
helm upgrade --install --wait podinfo --namespace podinfo \
--set serviceAccount.enabled=true \
--set tls.enabled=true \
--set spiffe.enabled=true \
--set spiffe.authorizedIDs={spiffe://example.org/ns/podinfo/sa/podinfo} \
oci://ghcr.io/stefanprodan/charts/podinfo
```

The Helm test client runs with podinfo's service account, so its ID is included in `authorizedIDs` above.

### Continuous Delivery

In order to install podinfo on a Kubernetes cluster and keep it up to date with the latest
release in an automated manner, you can use [Flux](https://fluxcd.io).

Install the Flux CLI on MacOS and Linux using Homebrew:

```sh
brew install fluxcd/tap/flux
```

Install the Flux controllers needed for Helm operations:

```sh
flux install \
--namespace=flux-system \
--network-policy=false \
--components=source-controller,helm-controller
```

Add podinfo's Helm repository to your cluster and
configure Flux to check for new chart releases every ten minutes:

```sh
flux create source helm podinfo \
--namespace=default \
--url=https://stefanprodan.github.io/podinfo \
--interval=10m
```

Create a `podinfo-values.yaml` file locally:

```sh
cat > podinfo-values.yaml <<EOL
replicaCount: 2
resources:
  limits:
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 64Mi
EOL
```

Create a Helm release for deploying podinfo in the default namespace:

```sh
flux create helmrelease podinfo \
--namespace=default \
--source=HelmRepository/podinfo \
--release-name=podinfo \
--chart=podinfo \
--chart-version=">5.0.0" \
--values=podinfo-values.yaml
```

Based on the above definition, Flux will upgrade the release automatically
when a new version of podinfo is released. If the upgrade fails, Flux
can [rollback](https://toolkit.fluxcd.io/components/helm/helmreleases/#configuring-failure-remediation)
to the previous working version.

You can check what version is currently deployed with:

```sh
flux get helmreleases -n default
```

To delete podinfo's Helm repository and release from your cluster run:

```sh
flux -n default delete source helm podinfo
flux -n default delete helmrelease podinfo
```

If you wish to manage the lifecycle of your applications in a **GitOps** manner, check out
this [workflow example](https://github.com/fluxcd/flux2-kustomize-helm-example)
for multi-env deployments with Flux, Kustomize and Helm.
