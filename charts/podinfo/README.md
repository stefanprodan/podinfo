# Podinfo

Podinfo is a tiny web application made with Go 
that showcases best practices of running microservices in Kubernetes.

## Installing the Chart

To install the chart with the release name `my-release`:

```console
$ helm repo add sp https://stefanprodan.github.io/podinfo
$ helm upgrade my-release --install sp/podinfo 
```

The command deploys podinfo on the Kubernetes cluster in the default namespace.
The [configuration](#configuration) section lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
$ helm delete --purge my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following tables lists the configurable parameters of the podinfo chart and their default values.

Parameter | Description | Default
--- | --- | ---
`affinity` | node/pod affinities | None
`backend` | echo backend URL | None
`backends` | echo backend URL array | None
`faults.delay` | random HTTP response delays between 0 and 5 seconds | `false`
`faults.error` | 1/3 chances of a random HTTP response error | `false`
`hpa.enabled` | enables HPA | `false`
`hpa.cpu` | target CPU usage per pod | None
`hpa.memory` | target memory usage per pod | None
`hpa.requests` | target requests per second per pod | None
`hpa.maxReplicas` | maximum pod replicas | `10`
`image.pullPolicy` | image pull policy | `IfNotPresent`
`image.repository` | image repository | `stefanprodan/podinfo`
`image.tag` | image tag | `<VERSION>`
`ingress.enabled` | enables ingress | `false`
`ingress.annotations` | ingress annotations | None
`ingress.hosts` | ingress accepted hostnames | None
`ingress.tls` | ingress TLS configuration | None
`message` | UI greetings message | None
`nodeSelector` | node labels for pod assignment | `{}`
`replicaCount` | desired number of pods | `2`
`resources.requests/cpu` | pod CPU request | `1m`
`resources.requests/memory` | pod memory request | `16Mi`
`resources.limits/cpu` | pod CPU limit | None
`resources.limits/memory` | pod memory limit | None
`service.enabled` | create Kubernetes service (should be disabled when using Flagger) | `true`
`service.metricsPort` | Prometheus metrics endpoint port | `9797`
`service.externalPort` | ClusterIP HTTP port | `9898`
`service.httpPort` | container HTTP port | `9898`
`service.nodePort` | NodePort for the HTTP endpoint | `31198`
`service.grpcPort` | ClusterIP gPRC port | `9999`
`service.grpcService` | gPRC service name | `podinfo`
`service.type` | type of service | `ClusterIP`
`tolerations` | list of node taints to tolerate | `[]`
`serviceAccount.enabled` | specifies whether a service account should be created | `false`
`serviceAccount.name` | the name of the service account to use, if not set and create is true, a name is generated using the fullname template | None
`linkerd.profile.enabled` | create Linkerd service profile | `false`

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```console
$ helm install stable/podinfo --name my-release \
  --set=image.tag=0.0.2,service.type=NodePort
```

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example,

```console
$ helm install stable/podinfo --name my-release -f values.yaml
```

> **Tip**: You can use the default [values.yaml](values.yaml)
```

