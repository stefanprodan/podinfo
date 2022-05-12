# Podinfo CUE module

This directory contains a [CUE](https://cuelang.org/docs/) module and tooling
for generating podinfo's Kubernetes resources.

The module contains a `podinfo.#Application` definition which takes `podinfo.#Config` as input.

## Prerequisites

Install CUE with:

```shell
brew install cue
```

Generate the Kubernetes API definitions required by this module with:

```shell
cue get go k8s.io/api/...
```

## Configuration

Configure the application in `main.cue`:

```cue
app: podinfo.#Application & {
	config: {
		meta: {
			name:      "podinfo"
			namespace: "default"
		}
		image: tag: "6.1.3"
		resources: requests: {
			cpu:    "100m"
			memory: "16Mi"
		}
		hpa: {
			enabled:     true
			maxReplicas: 3
		}
		ingress: {
			enabled:   true
			className: "nginx"
			host:      "podinfo.example.com"
			tls:       true
			annotations: "cert-manager.io/cluster-issuer": "letsencrypt"
		}
		serviceMonitor: enabled: true
	}
}
```

## Generate the manifests

```shell
cue gen
```
