# Podinfo

Podinfo is a tiny web application made with Go
that showcases best practices of running microservices in Kubernetes.

Podinfo is used by CNCF projects like [Flux](https://github.com/fluxcd/flux2)
and [Flagger](https://github.com/fluxcd/flagger)
for end-to-end testing and workshops.

## Installing the Chart

The Podinfo charts are published to
[GitHub Container Registry](https://github.com/stefanprodan/podinfo/pkgs/container/charts%2Fpodinfo)
and signed with [Cosign](https://github.com/sigstore/cosign) & GitHub Actions OIDC.

To install the chart with the release name `podinfo` from GHCR:

```console
$ helm upgrade -i podinfo oci://ghcr.io/stefanprodan/charts/podinfo
```

To verify a chart version with Cosign:

```console
$ cosign verify ghcr.io/stefanprodan/charts/podinfo:<VERSION> \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  --certificate-identity-regexp="^https://github\.com/stefanprodan/.*$"
```

Alternatively, you can install the chart from GitHub pages:

```console
$ helm repo add stefanprodan https://stefanprodan.github.io/podinfo

$ helm upgrade -i podinfo stefanprodan/podinfo
```

The command deploys podinfo on the Kubernetes cluster in the default namespace.
The [configuration](#configuration) section lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall the `podinfo` release:

```console
$ helm uninstall podinfo
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following tables lists the configurable parameters of the podinfo chart and their default values.

| Parameter                                      | Default                        | Description                                                                                        |
|------------------------------------------------|--------------------------------|----------------------------------------------------------------------------------------------------|
| `replicaCount`                                 | `1`                            | Desired number of pods                                                                             |
| `logLevel`                                     | `info`                         | Log level: `debug`, `info`, `warn`, `error`                                                        |
| `backend`                                      | `None`                         | Echo backend URL                                                                                   |
| `backends`                                     | `[]`                           | Array of echo backend URLs                                                                         |
| `cache`                                        | `None`                         | Redis or Valkey URL in the format `redis://<host>:<port>`, `rediss://` for TLS                     |
| `cacheOptions.cluster`                         | `false`                        | Connect in cluster mode, implied when the URL has more than one address                            |
| `cacheOptions.sentinelPrimary`                 | `""`                           | Name of the primary monitored by Sentinel, the URL addresses are then the Sentinels                |
| `cacheOptions.credentialsSecretName`           | `""`                           | Secret with the `PODINFO_CACHE_USERNAME`, `PODINFO_CACHE_PASSWORD` and `PODINFO_CACHE_SENTINEL_PASSWORD` keys |
| `cacheOptions.ca.secretName`                   | `""`                           | Secret holding the CA bundle that verifies the cache server certificate                            |
| `cacheOptions.ca.configMapName`                | `""`                           | ConfigMap holding the CA bundle, mutually exclusive with `cacheOptions.ca.secretName`              |
| `cacheOptions.ca.key`                          | `ca.crt`                       | Key of the CA bundle in the Secret or ConfigMap                                                    |
| `cacheOptions.cert.secretName`                 | `""`                           | `kubernetes.io/tls` Secret holding the client key pair presented to the cache server, a rotated pair is used for new connections |
| `redis.enabled`                                | `false`                        | Create Redis deployment for caching purposes                                                       |
| `redis.repository`                             | `docker.io/redis`              | Redis image repository                                                                             |
| `redis.tag`                                    | `<VERSION>`                    | Redis image tag                                                                                    |
| `redis.imagePullSecrets`                       | `[]`                           | Redis image pull secrets                                                                           |
| `redis.tls.enabled`                            | `false`                        | Serve Redis over mutual TLS with cert-manager certificates, podinfo presents a client certificate   |
| `redis.tls.issuerRef`                          | `{}`                           | CA issuer that signs the Redis and podinfo certificates, a CA created by the chart when empty      |
| `redis.tls.duration`                           | `8760h`                        | Lifetime of the Redis server certificate                                                           |
| `ui.color`                                     | `#1f4e8c`                      | UI color, green is `#17694f`                                                                       |
| `ui.message`                                   | `None`                         | UI greetings message                                                                               |
| `ui.logo`                                      | `None`                         | UI logo                                                                                            |
| `prefix`                                       | `/`                            | URL path prefix for HTTP routes (e.g. `/foo` maps `/api/info` to `/foo/api/info`)                 |
| `host`                                         | `None`                         | Host address to bind the HTTP and gRPC servers to, all interfaces when not set                      |
| `faults.delay`                                 | `false`                        | Random HTTP response delays between 0 and 5 seconds                                                |
| `faults.error`                                 | `false`                        | 1/3 chances of a random HTTP response error                                                        |
| `faults.unhealthy`                             | `false`                        | When set, the healthy state is never reached                                                       |
| `faults.unready`                               | `false`                        | When set, the ready state is never reached                                                         |
| `faults.testFail`                              | `false`                        | When set, a helm test is included which always fails                                               |
| `faults.testTimeout`                           | `false`                        | When set, a helm test is included which always times out                                           |
| `image.repository`                             | `ghcr.io/stefanprodan/podinfo` | Image repository                                                                                   |
| `image.tag`                                    | `<VERSION>`                    | Image tag                                                                                          |
| `image.pullPolicy`                             | `IfNotPresent`                 | Image pull policy                                                                                  |
| `image.pullSecrets`                            | `[]`                           | Image pull secrets                                                                                 |
| `service.enabled`                              | `true`                         | Create a Kubernetes Service, should be disabled when using [Flagger](https://flagger.app)          |
| `service.type`                                 | `ClusterIP`                    | Type of the Kubernetes Service                                                                     |
| `service.metricsPort`                          | `9797`                         | Prometheus metrics endpoint port                                                                   |
| `service.httpPort`                             | `9898`                         | Container HTTP port                                                                                |
| `service.externalPort`                         | `9898`                         | ClusterIP HTTP port                                                                                |
| `service.grpcPort`                             | `9999`                         | ClusterIP gPRC port                                                                                |
| `service.grpcService`                          | `podinfo`                      | gPRC service name                                                                                  |
| `service.grpcTLS`                              | `false`                        | Serve the gRPC port with TLS (SPIFFE mTLS when `spiffe.enabled`), requires `tls.enabled`           |
| `service.nodePort`                             | `31198`                        | NodePort for the HTTP endpoint                                                                     |
| `service.trafficDistribution`                  | `""`                           | Traffic distribution strategy                                                                      |
| `service.additionalLabels`                     | `{}`                           | Additional labels to add to the service                                                            |
| `service.externalTrafficPolicy`                | `None`                         | External traffic policy for LoadBalance service                                                    |
| `service.annotations`                          | `{}`                           | Annotations of the Kubernetes Service                                                               |
| `service.hostPort`                             | `None`                         | Bind the HTTP port to this host port                                                                |
| `h2c.enabled`                                  | `false`                        | Allow upgrading to h2c (non-TLS version of HTTP/2)                                                 |
| `tls.enabled`                                  | `false`                        | Serve HTTPS on a second port with the certificate from `tls.secretName`                             |
| `tls.secretName`                               | `None`                         | `kubernetes.io/tls` Secret with the certificate key pair, reloaded on rotation                      |
| `tls.certPath`                                 | `/data/cert`                   | Path where the certificate key pair is mounted                                                      |
| `tls.port`                                     | `9899`                         | HTTPS port of the container and the Service                                                         |
| `tls.hostPort`                                 | `None`                         | Bind the HTTPS port to this host port                                                               |
| `certificate.create`                           | `false`                        | Create a cert-manager Certificate for `tls.secretName`                                              |
| `certificate.issuerRef.kind`                   | `ClusterIssuer`                | Kind of the cert-manager issuer                                                                     |
| `certificate.issuerRef.name`                   | `self-signed`                  | Name of the cert-manager issuer                                                                     |
| `certificate.dnsNames`                         | `["podinfo"]`                  | Subject alternative names of the certificate                                                        |
| `spiffe.enabled`                               | `false`                        | Serve the TLS port with SPIFFE mTLS, requires `tls.enabled`, SPIRE and the SPIFFE CSI driver       |
| `spiffe.endpointSocket`                        | `unix:///spiffe-workload-api/spire-agent.sock` | SPIFFE Workload API socket                                                                         |
| `spiffe.csiDriver`                             | `csi.spiffe.io`                | CSI driver that mounts the Workload API socket                                                     |
| `spiffe.authorizedIDs`                         | `[]`                           | Client SPIFFE IDs allowed to connect, when empty any ID in the trust domain is allowed             |
| `spiffe.backendMTLS`                           | `false`                        | Call the `https` backends with SPIFFE mTLS                                                         |
| `spiffe.backendIDs`                            | `[]`                           | Server SPIFFE IDs accepted from `https` backends, when empty any ID in the trust domain is accepted |
| `extraArgs`                                    | `[]`                           | Additional command line arguments to pass to podinfo container                                     |
| `extraEnvs`                                    | `[]`                           | Extra environment variables for the podinfo container; an `OTEL_*` variable replaces the generated one |
| `otel.enabled`                                 | `false`                        | Export traces, metrics and logs over OTLP                                                          |
| `otel.endpoint`                                | `""`                           | OTLP endpoint, e.g. `http://otel-collector:4318`, required when `otel.enabled`                     |
| `otel.protocol`                                | `""`                           | OTLP protocol, `http/protobuf` (port 4318) when unset or `grpc` (port 4317)                        |
| `otel.serviceName`                             | `""`                           | Service name, defaults to the release fullname                                                     |
| `otel.resourceAttributes`                      | `{}`                           | Extra resource attributes, merged over the Kubernetes attributes set from the downward API         |
| `otel.sampler`                                 | `""`                           | Traces sampler, `parentbased_always_on` when unset                                                 |
| `otel.samplerArg`                              | `""`                           | Traces sampler argument, e.g. `0.1` for `parentbased_traceidratio`, requires `otel.sampler`        |
| `config.path`                                  | `""`                           | config file path                                                                                   |
| `config.name`                                  | `""`                           | config file name                                                                                   |
| `hpa.enabled`                                  | `false`                        | Enables the Kubernetes HPA                                                                         |
| `hpa.maxReplicas`                              | `10`                           | Maximum amount of pods                                                                             |
| `hpa.cpu`                                      | `None`                         | Target CPU usage per pod                                                                           |
| `hpa.memory`                                   | `None`                         | Target memory usage per pod                                                                        |
| `hpa.requests`                                 | `None`                         | Target HTTP requests per second per pod                                                            |
| `serviceAccount.enabled`                       | `false`                        | Whether a service account should be created                                                        |
| `serviceAccount.name`                          | `None`                         | The name of the service account to use, if not set a name is generated using the fullname template |
| `serviceAccount.imagePullSecrets`              | `[]`                           | List of image pull secrets if pulling from private registries                                      |
| `securityContext`                              | `{}`                           | The security context to be set on the podinfo container                                            |
| `podSecurityContext`                           | `{}`                           | The security context to be set on the pod                                                          |
| `podAnnotations`                               | `{}`                           | Pod annotations                                                                                    |
| `serviceMonitor.enabled`                       | `false`                        | Whether a Prometheus Operator service monitor should be created                                    |
| `serviceMonitor.interval`                      | `15s`                          | Prometheus scraping interval                                                                       |
| `serviceMonitor.additionalLabels`              | `{}`                           | Add additional labels to the service monitor                                                       |
| `ingress.enabled`                              | `false`                        | Enables Ingress                                                                                    |
| `ingress.className`                            | `""`                           | Use ingressClassName                                                                               |
| `ingress.additionalLabels`                     | `{}`                           | Add additional labels to the ingress                                                               |
| `ingress.annotations`                          | `{}`                           | Ingress annotations                                                                                |
| `ingress.hosts`                                | `[{host: podinfo.local, paths: [/]}]` | Ingress accepted hosts                                                                              |
| `ingress.tls`                                  | `[]`                           | Ingress TLS configuration                                                                          |
| `httpRoute.enabled`                            | `false`                        | Enables Gateway API HTTPRoute                                                                      |
| `httpRoute.additionalLabels`                   | `{}`                           | Add additional labels to the HTTPRoute                                                             |
| `httpRoute.annotations`                        | `{}`                           | HTTPRoute annotations                                                                              |
| `httpRoute.parentRefs`                         | `[{name: gateway, sectionName: http}]` | Gateways that this route is attached to                                                             |
| `httpRoute.hostnames`                          | `["podinfo.local"]`            | Hostnames matching HTTP header                                                                     |
| `httpRoute.rules`                              | `[{matches: [{path: /}]}]`     | List of rules and filters applied                                                                   |
| `grpcRoute.enabled`                            | `false`                        | Enables Gateway API GRPCRoute                                                                      |
| `grpcRoute.additionalLabels`                   | `{}`                           | Add additional labels to the GRPCRoute                                                             |
| `grpcRoute.annotations`                        | `{}`                           | GRPCRoute annotations                                                                              |
| `grpcRoute.parentRefs`                         | `[{name: gateway, sectionName: http}]` | Gateways that this route is attached to                                                             |
| `grpcRoute.hostnames`                          | `["podinfo.local"]`            | Hostnames matching HTTP header                                                                     |
| `grpcRoute.rules`                              | `[{}]`                         | List of rules with matches and filters applied, match all by default                               |
| `hooks.<hookType>.job.enabled`                 | `false`                        | Create a Helm hook job for testing (hookType: see values.yaml for available hooks)                 |
| `hooks.<hookType>.job.hookDeletePolicy`        | `hook-succeeded,hook-failed`   | Helm hook delete policy                                                                            |
| `hooks.<hookType>.job.ttlSecondsAfterFinished` | `None`                         | Job TTL after finished                                                                             |
| `hooks.<hookType>.job.sleepSeconds`            | `None`                         | Sleep duration before job exits                                                                    |
| `hooks.<hookType>.job.exitCode`                | `0`                            | Job exit code                                                                                      |
| `resources.requests.cpu`                       | `1m`                           | Pod CPU request                                                                                    |
| `resources.requests.memory`                    | `16Mi`                         | Pod memory request                                                                                 |
| `resources.limits.cpu`                         | `None`                         | Pod CPU limit                                                                                      |
| `resources.limits.memory`                      | `None`                         | Pod memory limit                                                                                   |
| `nodeSelector`                                 | `{}`                           | Node labels for pod assignment                                                                     |
| `tolerations`                                  | `[]`                           | List of node taints to tolerate                                                                    |
| `affinity`                                     | `{}`                           | Node/pod affinities                                                                                 |
| `topologySpreadConstraints`                    | `[]`                           | Pod topology spread constraints                                                                     |
| `podDisruptionBudget`                          | `{}`                           | PodDisruptionBudget spec, e.g. `maxUnavailable: 1`, applied when `replicaCount` is greater than 1   |
| `probes.readiness`                             | see `values.yaml`              | Delay, timeout, period and thresholds of the readiness probe                                        |
| `probes.liveness`                              | see `values.yaml`              | Delay, timeout, period and thresholds of the liveness probe                                         |
| `probes.startup.enable`                        | `false`                        | Add a startup probe, with its own delay, timeout, period and thresholds                             |
| `priorityClassName`                            | `""`                           | Pod priority class name                                                                            |

Specify each parameter using the `--set key=value[,key=value]` argument:

```console
$ helm upgrade -i podinfo oci://ghcr.io/stefanprodan/charts/podinfo \
  --set=serviceMonitor.enabled=true,serviceMonitor.interval=5s
```

To add custom annotations you need to escape the annotation key string:

```console
$ helm upgrade -i podinfo oci://ghcr.io/stefanprodan/charts/podinfo \
--set podAnnotations."toolkit\.fluxcd\.io\/tenant"=dev-team
```

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart:

```console
$ helm upgrade -i my-release oci://ghcr.io/stefanprodan/charts/podinfo -f values.yaml
```

## Cache

Podinfo caches to the bundled Redis (`redis.enabled`) or to an external Redis or Valkey server set with `cache`.

With `redis.tls.enabled`, the bundled Redis accepts only TLS clients that present a certificate signed by its CA.
The chart creates the CA and the Redis and podinfo certificates with cert-manager. Set `redis.tls.issuerRef`
to sign them with an existing private CA issuer instead (CA or Vault, not ACME), whose secrets carry the root CA as `ca.crt`.
`cacheOptions.ca` and `cacheOptions.cert` replace the CA bundle and the client key pair that podinfo uses.

Podinfo loads a renewed client certificate for new connections, but Redis reads its own certificate only at startup,
so `redis.tls.duration` is long and Redis must be restarted when cert-manager renews it.
Both read the CA at startup, so a renewed CA needs Redis and podinfo restarted.
Unless cert-manager runs with `--enable-certificate-owner-ref`, the certificate secrets stay after `helm uninstall`.

For an external server, the URL takes the extra cluster nodes or Sentinels as `addr` parameters:

```yaml
cache: "rediss://redis-0.redis:6379?addr=redis-1.redis:6379&addr=redis-2.redis:6379"
cacheOptions:
  cluster: true
  credentialsSecretName: redis-credentials
  ca:
    secretName: redis-tls
```

The client dials the addresses that the cluster nodes and Sentinels announce, not the hostnames in the URL.
With TLS, the servers must announce hostnames instead of pod IPs, or the certificate verification fails:

- Cluster: `cluster-announce-hostname <hostname>` and `cluster-preferred-endpoint-type hostname`
- Sentinel: `replica-announce-ip <hostname>` on the servers, `sentinel resolve-hostnames yes` and `sentinel announce-hostnames yes` on the Sentinels

## OpenTelemetry

With `otel.enabled`, podinfo exports traces, metrics and logs to the OTLP endpoint:

```yaml
otel:
  enabled: true
  endpoint: http://otel-collector.monitoring:4318
  resourceAttributes:
    deployment.environment.name: production
```

The resource carries the `k8s.namespace.name`, `k8s.deployment.name`, `k8s.container.name`, `k8s.pod.name`,
`k8s.pod.uid`, `k8s.node.name` and `service.namespace` attributes, set from the release and the downward API.
Any other [OTEL_*](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/) variable
is set with `extraEnvs`, which also overrides the generated ones:

```yaml
extraEnvs:
  - name: OTEL_EXPORTER_OTLP_HEADERS
    valueFrom:
      secretKeyRef:
        name: otlp-auth
        key: headers
  - name: OTEL_METRICS_EXPORTER
    value: none
```
