# CUE Demo

This directory contains a [cuelang module](https://cuelang.org/docs/) and tooling to generate podinfo resources.

It defines a `podinfo.#Application` definition which takes a `podinfo.#Config` as input.
The `podinfo.#Config` definition is modelled on the `podinfo` Helm chart `values.yaml` file.

## Prerequisites

Generate the Kubernetes API definitions required by podinfo with:

```shell
cue get go k8s.io/api/...
cue get go github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1
```

## Configuration

Configure the application in `main.cue`.

## Generate the manifests

```shell
cue gen
```
