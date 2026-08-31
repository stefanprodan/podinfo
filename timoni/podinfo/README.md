# Podinfo

[Podinfo](https://github.com/stefanprodan/podinfo) is a tiny web application
made with Go that showcases the best practices of running microservices in Kubernetes.

## Module Repository

This module is available on GitHub Container Registry at
[ghcr.io/stefanprodan/modules/podinfo](https://github.com/stefanprodan/podinfo/pkgs/container/modules%2Fpodinfo).

To list all available versions and their digests:

```shell
timoni mod list oci://ghcr.io/stefanprodan/modules/podinfo
```

## Prerequisites

- Kubernetes 1.25+
- [Timoni](https://timoni.sh/install/) 0.34+

## Install

To create an instance using the default values:

```shell
timoni -n default apply podinfo oci://ghcr.io/stefanprodan/modules/podinfo
```

To install a specific module version:

```shell
timoni -n default apply podinfo oci://ghcr.io/stefanprodan/modules/podinfo -v 6.14.1
```

To change the [default configuration](#configuration),
create one or more `values.cue` files and apply them to the instance.

For example, create a file `my-values.cue` with the following content:

```cue
values: {
	resources: requests: {
		cpu:    "100m"
		memory: "128Mi"
	}
}
```

And apply the values with:

```shell
timoni -n default apply podinfo oci://ghcr.io/stefanprodan/modules/podinfo \
--values ./my-values.cue
```

## Uninstall

To uninstall an instance and delete all its Kubernetes resources:

```shell
timoni -n default delete podinfo
```

## Bundle

For production deployments it is recommended to use a Timoni
[bundle](https://timoni.sh/bundle/) that offers a declarative way of
managing the lifecycle of applications and their infra dependencies.

The following bundle deploys a highly-available podinfo frontend
exposed through a Gateway API HTTPRoute, calling a backend instance
with Redis caching enabled:

```cue
bundle: {
	apiVersion: "v1alpha1"
	name:       "podinfo"
	instances: {
		backend: {
			module: url: "oci://ghcr.io/stefanprodan/modules/podinfo"
			namespace: "podinfo"
			values: {
				caching: {
					enabled:  true
					redisURL: "tcp://redis:6379"
				}
				hpa: {
					enabled:     true
					minReplicas: 2
					maxReplicas: 10
					cpu:         90
				}
			}
		}
		frontend: {
			module: url: "oci://ghcr.io/stefanprodan/modules/podinfo"
			namespace: "podinfo"
			values: {
				ui: backend: "http://backend.podinfo.svc.cluster.local/echo"
				replicas: 2
				podDisruptionBudget: enabled: true
				httpRoute: {
					enabled: true
					parentRefs: [{
						name:      "gateway"
						namespace: "gateway-system"
					}]
					hostnames: ["podinfo.example.com"]
				}
				test: enabled: true
			}
		}
	}
}
```

Save the bundle as `podinfo.cue` and apply the stack with:

```shell
timoni bundle apply -f podinfo.cue
```

## Configuration

All values are optional. The module complies with the restricted
[Kubernetes pod security standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
by default: the container is denied privilege escalation, its root
filesystem is read-only, all its capabilities are dropped, and the pod
runs as the non-root user of the podinfo image with the `RuntimeDefault`
seccomp profile.

### General values

| Key | Type | Default | Description |
|---|---|---|---|
| `image` | `timoniv1.#Image` | `ghcr.io/stefanprodan/podinfo:<latest version>` | Container image repository, tag, digest and pull policy; the digest takes precedence over the tag when specified |
| `metadata.labels` / `annotations` | `{[string]: string}` | unset | Extra labels and annotations added to all resources |
| `replicas` | `int` | `1` | Number of pod replicas; ignored when `hpa` is enabled |
| `revisionHistoryLimit` | `int` | unset | Number of old ReplicaSets to retain |
| `strategy` | `appsv1.#DeploymentStrategy` | `RollingUpdate` with `maxUnavailable: 50%` | Rollout strategy |
| `logLevel` | `string` | `info` | Log level: `debug`, `info`, `warn` or `error` |
| `extraArgs` | `[...string]` | `[]` | Extra command line arguments appended to the podinfo command |
| `env` | `[...corev1.#EnvVar]` | unset | Extra environment variables of the podinfo container |
| `resources` | `timoniv1.#ResourceRequirements` | `10m` CPU, `32Mi` memory requests | Container resource requirements |
| `securityContext` | `corev1.#SecurityContext` | hardened | Container security context; defaults: no privilege escalation, read-only rootfs, all capabilities dropped (the pod identity comes from `podSecurityContext`) |
| `livenessProbe` / `readinessProbe` | `corev1.#Probe` | `/healthz`, `/readyz` | Container probes on the http port |
| `startupProbe` | `corev1.#Probe` | unset | Startup probe |
| `dataVolume` | `corev1.#VolumeSource` | `emptyDir` | Volume backing `/data`, where podinfo stores its runtime data (the root filesystem is read-only) |
| `extraVolumes` / `extraVolumeMounts` | `[...]` | unset | Additional volumes and volume mounts |
| `initContainers` / `extraContainers` | `[...corev1.#Container]` | unset | Additional containers |
| `serviceAccount.create` | `bool` | `true` | Create the service account; set to `false` to use an existing one |
| `serviceAccount.name` | `string` | instance name, or `default` when `create: false` | Service account name |
| `serviceAccount.labels` / `annotations` | `{[string]: string}` | unset | Extra service account metadata (e.g. IRSA role annotations) |
| `serviceAccount.automountServiceAccountToken` | `bool` | `false` | Mount the token through the service account (the pod setting mounts it instead) |
| `test.enabled` | `bool` | `false` | Run the end-to-end test Jobs at install and upgrades: a curl request to the http port and a grpc-health-probe check of the grpc port |
| `test.image` | `timoniv1.#Image` | `ghcr.io/curl/curl-container/curl-multi:<version>` | Container image of the http test Job |
| `test.grpcImage` | `timoniv1.#Image` | `ghcr.io/grpc-ecosystem/grpc-health-probe:<version>` | Container image of the gRPC test Job |

### Pod scheduling values

| Key | Type | Default | Description |
|---|---|---|---|
| `podLabels` / `podAnnotations` | `{[string]: string}` | unset | Extra pod metadata |
| `securityContextPreset` | `hardened` or `platform` | `hardened` | Pod identity defaults: `hardened` pins the image's non-root UID `100` and GID `101`, `platform` leaves the identity to the cluster (e.g. OpenShift SCCs) |
| `podSecurityContext` | `corev1.#PodSecurityContext` | per `securityContextPreset` | Pod security context |
| `imagePullSecrets` | `[...]` | unset | Secrets for pulling from private registries |
| `priorityClassName` | `string` | unset | Pod priority class |
| `affinity.podAntiAffinity` | `soft`, `hard`, `none` or raw rules | `soft` | Spread the replicas across nodes; raw rules replace the preset and need explicit label selectors |
| `affinity.nodeAffinity` / `affinity.podAffinity` | raw rules | unset | Node and pod affinity rules |
| `nodeSelector` | `{[string]: string}` | Linux nodes | Node selection; a supplied value replaces the default |
| `tolerations` / `topologySpreadConstraints` | | unset | Standard scheduling controls |
| `hostAliases` | `[...corev1.#HostAlias]` | unset | Extra entries for the pod /etc/hosts |
| `schedulerName` | `string` | unset | Alternate scheduler |
| `dnsPolicy` / `dnsConfig` | | unset | Pod DNS settings |
| `terminationGracePeriodSeconds` | `int` | unset | Pod termination grace period |
| `automountServiceAccountToken` | `bool` | `false` | Mount the API credentials in the pod; podinfo does not talk to the Kubernetes API |
| `deploymentLabels` / `deploymentAnnotations` | `{[string]: string}` | unset | Extra Deployment metadata |

### Service values

The http-metrics port (`9797`) is always exposed for scraping.

| Key | Type | Default | Description |
|---|---|---|---|
| `service.port` | `int` | `80` | Service http port, the backend of `ingress` and `httpRoute` |
| `service.grpcPort` | `int` | `9999` | Service grpc port, targeting the podinfo gRPC server |
| `service.type` | `string` | `ClusterIP` | Service type |
| `service.annotations` / `labels` | `{[string]: string}` | unset | Extra Service metadata |
| `service.clusterIP` / `externalIPs` / `loadBalancerIP` / `loadBalancerClass` / `loadBalancerSourceRanges` / `externalTrafficPolicy` | | unset | Service networking settings per type |
| `service.nodePort` / `grpcNodePort` | `int` | `0` (auto) | Node ports with the `NodePort` and `LoadBalancer` service types |
| `service.ipFamilies` / `ipFamilyPolicy` | | unset | Service IP family settings |

### Routing values

| Key | Type | Default | Description |
|---|---|---|---|
| `ingress.enabled` | `bool` | `false` | Create an Ingress for the http Service port |
| `ingress.className` | `string` | unset | IngressClass name |
| `ingress.hosts` | `[...#IngressHost]` | required when enabled | Ingress rules: each host takes `paths` with `path` and `pathType` (default `/` `Prefix`) |
| `ingress.tls` | `[...networkingv1.#IngressTLS]` | unset | TLS termination settings |
| `ingress.annotations` / `labels` | `{[string]: string}` | unset | Extra Ingress metadata |
| `httpRoute.enabled` | `bool` | `false` | Create a Gateway API HTTPRoute for the http Service port |
| `httpRoute.parentRefs` | `[...]` | required when enabled | Gateways the route attaches to (`name`, `namespace`, optional `sectionName` to pin a listener); TLS is terminated by the Gateway listener |
| `httpRoute.hostnames` | `[...string]` | unset | Hostnames the route matches |
| `httpRoute.rules` | `[...]` | match all | Route rules with `matches` and `filters`; the backend reference is generated by the module |
| `httpRoute.annotations` / `labels` | `{[string]: string}` | unset | Extra HTTPRoute metadata |

For example, to expose podinfo with an Ingress and a cert-manager issued certificate:

```cue
values: {
	ingress: {
		enabled:   true
		className: "nginx"
		annotations: "cert-manager.io/cluster-issuer": "letsencrypt"
		hosts: [{host: "podinfo.example.com"}]
		tls: [{
			secretName: "podinfo-tls"
			hosts: ["podinfo.example.com"]
		}]
	}
}
```

Or with a Gateway API HTTPRoute attached to an existing Gateway. TLS is
terminated by the Gateway listener the route attaches to (the certificate
is referenced in the Gateway spec, not in the route), and the route
hostnames must match the listener hostname:

```cue
values: {
	httpRoute: {
		enabled: true
		parentRefs: [{
			name:      "gateway"
			namespace: "gateway-system"
		}]
		hostnames: ["podinfo.example.com"]
	}
}
```

When waiting for the instance to become ready, Timoni waits for the
HTTPRoute to be accepted by the referenced Gateways.

### Autoscaling and disruption values

| Key | Type | Default | Description |
|---|---|---|---|
| `hpa.enabled` | `bool` | `false` | Create a HorizontalPodAutoscaler owning the replica count |
| `hpa.minReplicas` / `maxReplicas` | `int` | `1` / `minReplicas` | Autoscaling bounds |
| `hpa.cpu` | `int` | `99` | CPU average utilization target (percentage); `0` disables the CPU metric |
| `hpa.memory` | `string` | unset | Memory average value target (e.g. `1024Mi`) |
| `hpa.metrics` | `[...]` | `[]` | Extra scaling metrics appended to the generated ones |
| `hpa.behavior` | | unset | Scaling behavior tuning |
| `podDisruptionBudget.enabled` | `bool` | `false` | Create a PodDisruptionBudget |
| `podDisruptionBudget.minAvailable` | `int` or percent | `1` | Minimum available pods; mutually exclusive with `maxUnavailable` |
| `podDisruptionBudget.maxUnavailable` | `int` or percent | unset | Maximum unavailable pods |
| `podDisruptionBudget.unhealthyPodEvictionPolicy` | `string` | unset | `IfHealthyBudget` or `AlwaysAllow`; requires Kubernetes 1.27+ |

### Monitoring values

When the ServiceMonitor is disabled, the metrics endpoint is advertised
to Prometheus through the `prometheus.io/scrape` and `prometheus.io/port`
pod annotations.

| Key | Type | Default | Description |
|---|---|---|---|
| `serviceMonitor.enabled` | `bool` | `false` | Create a Prometheus Operator ServiceMonitor for the http-metrics Service port, in the instance namespace |
| `serviceMonitor.additionalLabels` / `annotations` | `{[string]: string}` | unset | Extra ServiceMonitor metadata, e.g. labels for Prometheus discovery |
| `serviceMonitor.jobLabel` | `string` | `app.kubernetes.io/name` | Service label used as the Prometheus job name |
| `serviceMonitor.interval` / `scrapeTimeout` | `string` | unset | Scrape cadence; defaults to the Prometheus settings |
| `serviceMonitor.honorLabels` | `bool` | `false` | Keep scraped label values on collision |
| `serviceMonitor.enableHttp2` | `bool` | unset | Enable HTTP2 for scraping |
| `serviceMonitor.scheme` / `tlsConfig` / `bearerTokenFile` / `bearerTokenSecret` / `proxyUrl` | | unset | Scrape scheme, TLS, authentication and proxy settings |
| `serviceMonitor.metricRelabelings` / `relabelings` | `[...]` | unset | Relabeling rules for the samples and the targets |
| `serviceMonitor.sampleLimit` / `targetLimit` / `labelLimit` / `labelNameLengthLimit` / `labelValueLengthLimit` | `int` | unset | Scrape limits |
| `serviceMonitor.targetLabels` / `podTargetLabels` | `[...string]` | unset | Service/pod labels copied onto the metrics |

### Caching values

| Key | Type | Default | Description |
|---|---|---|---|
| `caching.enabled` | `bool` | `false` | Enable Redis caching |
| `caching.redisURL` | `string` | required when enabled | Redis URL in the format `tcp://[:password@]host[:port]` |

### UI values

| Key | Type | Default | Description |
|---|---|---|---|
| `ui.color` | `string` | `#34577c` | Background color |
| `ui.message` | `string` | unset | Greeting message |
| `ui.backend` | `string` | unset | Backend service URL called by the `/api/echo` endpoint |
