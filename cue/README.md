# CUE Demo

This directory contains a [cuelang module](https://cuelang.org/docs/) and tooling to generate podinfo resources.

It defines a `podinfo.#Application` definition which takes a `podinfo.#Config` as input. The `podinfo.#Config` definition is modelled on the `podinfo` Helm chart `values.yaml` file.

## Configuration

Configure the application in `main.cue`.

## Generate the manifests

```shell
cue gen
```
