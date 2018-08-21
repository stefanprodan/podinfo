# Podinfo

Podinfo is a tiny web application made with Go 
that showcases best practices of running microservices in Kubernetes.

## Installing the Chart

To install the chart with the release name `my-release`:

```console
$ helm repo add sp https://stefanprodan.github.io/k8s-podinfo
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
`color` | UI color | blue
`backend` | echo backend URL | None
`faults.delay` | random HTTP response delays between 0 and 5 seconds | `false`
`faults.error` | 1/3 chances of a random HTTP response error | `false`
`hpa.enabled` | enables HPA | `false`
`hpa.cpu` | target CPU usage per pod | None
`hpa.memory` | target memory usage per pod | None
`hpa.requests` | target requests per second per pod | None
`hpa.maxReplicas` | maximum pod replicas | `10`
`ingress.hosts` | ingress accepted hostnames | None
`ingress.tls` | ingress TLS configuration | None:
`image.pullPolicy` | image pull policy | `IfNotPresent`
`image.repository` | image repository | `stefanprodan/podinfo`
`image.tag` | image tag | `0.0.1`
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
`service.externalPort` | external port for the service | `9898`
`service.internalPort` | internal port for the service | `9898`
`service.nodePort` | node port for the service | `31198`
`service.type` | type of service | `ClusterIP`
`tolerations` | list of node taints to tolerate | `[]`

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

