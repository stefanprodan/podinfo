# Podinfo Installation

To install Podinfo on Kubernetes the minimum required version is **Kubernetes v1.23**
with Helm and Kustomize, and **Kubernetes v1.25** with Timoni.

## Timoni

Install with [Timoni](https://timoni.sh):

```bash
timoni -n default apply podinfo oci://ghcr.io/stefanprodan/modules/podinfo
```

## Helm

Install from github.io:

```bash
helm repo add podinfo https://stefanprodan.github.io/podinfo

helm upgrade --install --wait frontend \
--namespace test \
--set replicaCount=2 \
--set backend=http://backend-podinfo:9898/echo \
podinfo/podinfo

helm test frontend --namespace test

helm upgrade --install --wait backend \
--namespace test \
--set redis.enabled=true \
podinfo/podinfo
```

Install from ghcr.io:

```bash
helm upgrade --install --wait podinfo --namespace default \
oci://ghcr.io/stefanprodan/charts/podinfo
```

## Kustomize

```bash
kubectl apply -k github.com/stefanprodan/podinfo//kustomize
```

## Docker

```bash
docker run -dp 9898:9898 stefanprodan/podinfo
```

## Continuous Delivery

[Flux](https://fluxcd.io) can install podinfo from its Helm chart and upgrade it when a new version is released.

Install the [Flux CLI](https://fluxcd.io/flux/installation/#install-the-flux-cli), then install the controllers needed for Helm operations:

```sh
flux install --components=source-controller,helm-controller
```

Configure Flux to check for new chart versions every ten minutes:

```sh
flux create source oci podinfo \
--namespace=default \
--url=oci://ghcr.io/stefanprodan/charts/podinfo \
--tag-semver=">=6.0.0" \
--interval=10m
```

Create a Helm release that deploys the chart in the default namespace:

```sh
flux create helmrelease podinfo \
--namespace=default \
--chart-ref=OCIRepository/podinfo.default
```

Check the deployed version with:

```sh
flux get helmreleases -n default
```

To remove podinfo from the cluster run:

```sh
flux -n default delete helmrelease podinfo
flux -n default delete source oci podinfo
```

To manage multiple environments with GitOps, see the Flux
[Kustomize and Helm example](https://github.com/fluxcd/flux2-kustomize-helm-example).
