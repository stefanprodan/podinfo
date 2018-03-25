# Ngrok

Expose Kubernetes service with [Ngrok](https://ngrok.com).

## Installing the Chart

To install the chart with the release name `my-release`:

```console
$ helm install sp/ngrok --name my-release \
  --set token=NGROK-TOKEN \
  --set expose.service=podinfo:9898
```

The command deploys Ngrok on the Kubernetes cluster in the default namespace.
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
`image.repository` | Image repository | `stefanprodan/ngrok`
`image.pullPolicy` | Image pull policy | `IfNotPresent`
`image.tag` | Image tag | `latest`
`replicaCount` | desired number of pods | `1`
`tolerations` | List of node taints to tolerate | `[]`
`affinity` | node/pod affinities | `node`
`nodeSelector` | node labels for pod assignment | `{}`
`service.type` | type of service | `ClusterIP`
`token` | Ngrok auth token | `none`
`expose.service` | Service address to be exposed as in `service-name:port` | `none`

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```console
$ helm upgrade --install --wait tunel \
  --set token=NGROK-TOKEN \
  --set service.type=NodePort \
  --set expose.service=podinfo:9898 \
  sp/ngrok
```

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example,

```console
$ helm install sp/grafana --name my-release -f values.yaml
```

> **Tip**: You can use the default [values.yaml](values.yaml)
```

