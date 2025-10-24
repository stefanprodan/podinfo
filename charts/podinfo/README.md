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
  --certificate-identity-regexp=^https://github\\.com/stefanprodan/podinfo/.*$
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

| Parameter                         | Default                        | Description                                                                                                            |
|-----------------------------------|--------------------------------|------------------------------------------------------------------------------------------------------------------------|
| `replicaCount`                    | `1`                            | Desired number of pods                                                                                                 |
| `logLevel`                        | `info`                         | Log level: `debug`, `info`, `warn`, `error`                                                                            |
| `backend`                         | `None`                         | Echo backend URL                                                                                                       |
| `backends`                        | `[]`                           | Array of echo backend URLs                                                                                             |
| `cache`                           | `None`                         | Redis address in the format `tcp://<host>:<port>`                                                                      |
| `redis.enabled`                   | `false`                        | Create Redis deployment for caching purposes                                                                           |
| `redis.repository`                | `docker.io/redis`              | Redis image repository                                                                                                 |
| `redis.tag`                       | `<VERSION>`                    | Redis image tag                                                                                                        |
| `redis.imagePullSecrets`          | `[]`                           | Redis image pull secrets                                                                                               |
| `ui.color`                        | `#34577c`                      | UI color                                                                                                               |
| `ui.message`                      | `None`                         | UI greetings message                                                                                                   |
| `ui.logo`                         | `None`                         | UI logo                                                                                                                |
| `faults.delay`                    | `false`                        | Random HTTP response delays between 0 and 5 seconds                                                                    |
| `faults.error`                    | `false`                        | 1/3 chances of a random HTTP response error                                                                            |
| `faults.unhealthy`                | `false`                        | When set, the healthy state is never reached                                                                           |
| `faults.unready`                  | `false`                        | When set, the ready state is never reached                                                                             |
| `faults.testFail`                 | `false`                        | When set, a helm test is included which always fails                                                                   |
| `faults.testTimeout`              | `false`                        | When set, a helm test is included which always times out                                                               |
| `image.repository`                | `ghcr.io/stefanprodan/podinfo` | Image repository                                                                                                       |
| `image.tag`                       | `<VERSION>`                    | Image tag                                                                                                              |
| `image.pullPolicy`                | `IfNotPresent`                 | Image pull policy                                                                                                      |
| `image.pullSecrets`               | `[]`                           | Image pull secrets                                                                                                     |
| `service.enabled`                 | `true`                         | Create a Kubernetes Service, should be disabled when using [Flagger](https://flagger.app)                              |
| `service.type`                    | `ClusterIP`                    | Type of the Kubernetes Service                                                                                         |
| `service.metricsPort`             | `9797`                         | Prometheus metrics endpoint port                                                                                       |
| `service.httpPort`                | `9898`                         | Container HTTP port                                                                                                    |
| `service.externalPort`            | `9898`                         | ClusterIP HTTP port                                                                                                    |
| `service.grpcPort`                | `9999`                         | ClusterIP gPRC port                                                                                                    |
| `service.grpcService`             | `podinfo`                      | gPRC service name                                                                                                      |
| `service.nodePort`                | `31198`                        | NodePort for the HTTP endpoint                                                                                         |
| `h2c.enabled`                     | `false`                        | Allow upgrading to h2c (non-TLS version of HTTP/2)                                                                     |
| `extraArgs`                       | `[]`                           | Additional command line arguments to pass to podinfo container                                                         |
| `extraEnvs`                       | `[]`                           | Extra environment variables for the podinfo container                                                                  |
| `config.path`                     | `""`                           | config file path                                                                                                       |
| `config.name`                     | `""`                           | config file name                                                                                                       |
| `hpa.enabled`                     | `false`                        | Enables the Kubernetes HPA                                                                                             |
| `hpa.maxReplicas`                 | `10`                           | Maximum amount of pods                                                                                                 |
| `hpa.cpu`                         | `None`                         | Target CPU usage per pod                                                                                               |
| `hpa.memory`                      | `None`                         | Target memory usage per pod                                                                                            |
| `hpa.requests`                    | `None`                         | Target HTTP requests per second per pod                                                                                |
| `serviceAccount.enabled`          | `false`                        | Whether a service account should be created                                                                            |
| `serviceAccount.name`             | `None`                         | The name of the service account to use, if not set and create is true, a name is generated using the fullname template |
| `serviceAccount.imagePullSecrets` | `[]`                           | List of image pull secrets if pulling from private registries.                                                         |
| `securityContext`                 | `{}`                           | The security context to be set on the podinfo container                                                                |
| `podSecurityContext`              | `{}`                           | The security context to be set on the pod                                                                              |
| `podAnnotations`                  | `{}`                           | Pod annotations                                                                                                        |
| `serviceMonitor.enabled`          | `false`                        | Whether a Prometheus Operator service monitor should be created                                                        |
| `serviceMonitor.interval`         | `15s`                          | Prometheus scraping interval                                                                                           |
| `serviceMonitor.additionalLabels` | `{}`                           | Add additional labels to the service monitor                                                                           |
| `ingress.enabled`                 | `false`                        | Enables Ingress                                                                                                        |
| `ingress.className`               | `""`                           | Use ingressClassName                                                                                                   |
| `ingress.additionalLabels`        | `{}`                           | Add additional labels to the ingress                                                                                   |
| `ingress.annotations`             | `{}`                           | Ingress annotations                                                                                                    |
| `ingress.hosts`                   | `[]`                           | Ingress accepted hosts                                                                                                 |
| `ingress.tls`                     | `[]`                           | Ingress TLS configuration                                                                                              |
| `httpRoute.enabled`               | `false`                        | Enables Gateway API HTTPRoute                                                                                          |
| `httpRoute.additionalLabels`      | `{}`                           | Add additional labels to the HTTPRoute                                                                                 |
| `httpRoute.annotations`           | `{}`                           | HTTPRoute annotations                                                                                                  |
| `httpRoute.parentRefs`            | `[]`                           | Gateways that this route is attached to                                                                                |
| `httpRoute.hostnames`             | `["podinfo.local"]`            | Hostnames matching HTTP header                                                                                         |
| `httpRoute.rules`                 | `[]`                           | List of rules and filters applied                                                                                      |
| `linkerd.profile.enabled`         | `false`                        | Create Linkerd service profile                                                                                         |
| `resources.requests.cpu`          | `1m`                           | Pod CPU request                                                                                                        |
| `resources.requests.memory`       | `16Mi`                         | Pod memory request                                                                                                     |
| `resources.limits.cpu`            | `None`                         | Pod CPU limit                                                                                                          |
| `resources.limits.memory`         | `None`                         | Pod memory limit                                                                                                       |
| `nodeSelector`                    | `{}`                           | Node labels for pod assignment                                                                                         |
| `tolerations`                     | `[]`                           | List of node taints to tolerate                                                                                        |
| `affinity`                        | `None`                         | Node/pod affinities                                                                                                    |

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
