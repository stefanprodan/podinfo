# Podinfo

Podinfo is a tiny web application made with Go 
that showcases best practices of running microservices in Kubernetes.

## Installing the Chart

To install the chart with the release name `my-release`:

```console
$ helm repo add podinfo https://stefanprodan.github.io/podinfo

$ helm upgrade -i my-release podinfo/podinfo 
```

The command deploys podinfo on the Kubernetes cluster in the default namespace.
The [configuration](#configuration) section lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
$ helm delete my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following tables lists the configurable parameters of the podinfo chart and their default values.

Parameter | Default | Description
--- | --- | ---
`replicaCount` | `1` | Desired number of pods
`logLevel` | `info` | Log level: `debug`, `info`, `warn`, `error`, `flat` or `panic`
`backend` | `None` | Echo backend URL
`backends` | `[]` | Array of echo backend URLs
`cache` | `None` | Redis address in the format `<host>:<port>`
`redis.enabled` | `false` | Create Redis deployment for caching purposes
`ui.color` | `#34577c` |  UI color
`ui.message` | `None` |  UI greetings message
`ui.logo` | `None` |  UI logo
`faults.delay` | `false` | Random HTTP response delays between 0 and 5 seconds
`faults.error` | `false` | 1/3 chances of a random HTTP response error
`faults.unhealthy` | `false` | When set, the healthy state is never reached
`faults.unready` | `false` | When set, the ready state is never reached
`faults.testFail` | `false` | When set, a helm test is included which always fails
`faults.testTimeout` | `false` | When set, a helm test is included which always times out
`h2c.enabled` | `false` | Allow upgrading to h2c
`image.repository` | `stefanprodan/podinfo` | Image repository
`image.tag` | `<VERSION>` | Image tag
`image.pullPolicy` | `IfNotPresent` | Image pull policy
`service.enabled` | `true` | Create a Kubernetes Service, should be disabled when using [Flagger](https://flagger.app)
`service.type` | `ClusterIP` | Type of the Kubernetes Service
`service.metricsPort` | `9797` | Prometheus metrics endpoint port
`service.httpPort` | `9898` | Container HTTP port
`service.externalPort` | `9898` | ClusterIP HTTP port
`service.grpcPort` | `9999` | ClusterIP gPRC port
`service.grpcService` | `podinfo` | gPRC service name
`service.nodePort` | `31198` | NodePort for the HTTP endpoint
`hpa.enabled` | `false` | Enables the Kubernetes HPA
`hpa.maxReplicas` | `10` | Maximum amount of pods
`hpa.cpu` | `None` | Target CPU usage per pod
`hpa.memory` | `None` | Target memory usage per pod
`hpa.requests` | `None` | Target HTTP requests per second per pod
`serviceAccount.enabled` | `false` | Whether a service account should be created
`serviceAccount.name` | `None` | The name of the service account to use, if not set and create is true, a name is generated using the fullname template
`linkerd.profile.enabled` | `false` | Create Linkerd service profile
`serviceMonitor.enabled` | `false` | Whether a Prometheus Operator service monitor should be created
`serviceMonitor.interval` | `15s` | Prometheus scraping interval
`ingress.enabled` | `false` | Enables Ingress
`ingress.annotations` | `{}` | Ingress annotations
`ingress.path` | `/*` | Ingress path
`ingress.hosts` | `[]` | Ingress accepted hosts
`ingress.tls` | `[]` | Ingress TLS configuration
`resources.requests.cpu` | `1m` | Pod CPU request
`resources.requests.memory` | `16Mi` | Pod memory request
`resources.limits.cpu` | `None` | Pod CPU limit
`resources.limits.memory` | `None` | Pod memory limit
`nodeSelector` | `{}` | Node labels for pod assignment
`tolerations` | `[]` | List of node taints to tolerate
`affinity` | `None` | Node/pod affinities
`podAnnotations` | `{}` | Pod annotations

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```console
$ helm install my-release podinfo/podinfo \
  --set=serviceMonitor.enabled=true,serviceMonitor.interval=5s
```

To add custom annotations you need to escape the annotation key string:

```console
$ helm upgrade -i my-release podinfo/podinfo \
--set podAnnotations."appmesh\.k8s\.aws\/preview"=enabled
```

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example,

```console
$ helm install my-release podinfo/podinfo -f values.yaml
```

> **Tip**: You can use the default [values.yaml](values.yaml)

## Upgrading the chart

### To =< 5.0.0

Version 5.0.0 is a major update.

* The chart now follows the new Kubernetes label recommendations:
<https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/>

The simplest way to update is to do a force upgrade, which recreates the resources by doing a delete and an install.
