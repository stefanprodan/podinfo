# Weave Cloud Grafana

Grafana v5 with Kubernetes dashboards and Prometheus and Weave Cloud data sources.

## Installing the Chart

To install the chart with the release name `my-release`:

```console
$ helm install stable/grafana --name my-release \
    --set service.type=NodePort \
    --set token=WEAVE-TOKEN \
    --set password=admin
```

The command deploys Grafana on the Kubernetes cluster in the default namespace.
The [configuration](#configuration) section lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
$ helm delete --purge my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following tables lists the configurable parameters of the Grafana chart and their default values.

Parameter | Description | Default
--- | --- | ---
`image.repository` | Image repository | `grafana/grafana`
`image.pullPolicy` | Image pull policy | `IfNotPresent`
`image.tag` | Image tag | `5.0.1`
`replicaCount` | desired number of pods | `1`
`resources` | pod resources | `none`
`tolerations` | List of node taints to tolerate | `[]`
`affinity` | node/pod affinities | `node`
`nodeSelector` | node labels for pod assignment | `{}`
`service.type` | type of service | `LoadBalancer`
`url` | Prometheus URL, used when Weave token is empty | `http://prometheus:9090`
`token` | Weave Cloud token | `none`
`user` | Grafana admin username | `admin`
`password` | Grafana admin password | `none`

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```console
$ helm install stable/grafana --name my-release \
  --set=token=WEAVE-TOKEN \
  --set password=admin
```

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example,

```console
$ helm install stable/grafana --name my-release -f values.yaml
```

> **Tip**: You can use the default [values.yaml](values.yaml)
```

