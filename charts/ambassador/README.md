# Ambassador

Ambassador is an open source, Kubernetes-native [microservices API gateway](https://www.getambassador.io/about/microservices-api-gateways) built on the [Envoy Proxy](https://www.envoyproxy.io/). 

## TL;DR;

```console
$ helm install stable/ambassador
```

## Introduction

This chart bootstraps an [Ambassador](https://www.getambassador.io) deployment on 
a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.7+

## Installing the Chart

To install the chart with the release name `my-release`:

```console
$ helm install --name my-release stable/ambassador
```

The command deploys Ambassador API gateway on the Kubernetes cluster in the default configuration. 
The [configuration](#configuration) section lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
$ helm delete --purge my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following tables lists the configurable parameters of the Ambassador chart and their default values.

| Parameter                       | Description                                | Default                                                    |
| ------------------------------- | ------------------------------------------ | ---------------------------------------------------------- |
| `image` | Image | `quay.io/datawire/ambassador` 
| `imageTag` | Image tag | `0.29.0` 
| `imagePullPolicy` | Image pull policy | `IfNotPresent` 
| `replicaCount`  | Number of Ambassador replicas  | `1` 
| `resources` | CPU/memory resource requests/limits | None 
| `rbac.create` | If `true`, create and use RBAC resources | `true`
| `serviceAccount.create` | If `true`, create a new service account | `true`
| `serviceAccount.name` | Service account to be used | `ambassador`
| `service.type` | Service type to be used | `LoadBalancer`
| `adminService.create` | If `true`, create a service for Ambassador's admin UI | `true`
| `adminService.type` | Ambassador's admin service type to be used | `ClusterIP`
| `exporter.image` | Prometheus exporter image | `datawire/prom-statsd-exporter:0.6.0`
| `timing.restart` | The minimum number of seconds between Envoy restarts | `15`
| `timing.drain` | The number of seconds that the Envoy will wait for open connections to drain on a restart | `5`
| `timing.shutdown` | The number of seconds that Ambassador will wait for the old Envoy to clean up and exit on a restart | `10`


