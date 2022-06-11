# Podinfo signed releases

Podinfo deployment manifests are published to GitHub Container Registry as OCI artifacts
and are signed using [cosign](https://github.com/sigstore/cosign).

## Verify the artifacts with cosign

Install the [cosign](https://github.com/sigstore/cosign) CLI:

```sh
brew install sigstore/tap/cosign
```

Verify a podinfo release with cosign CLI:

```sh
cosign verify -key https://raw.githubusercontent.com/stefanprodan/podinfo/master/cosign/cosign.pub \
ghcr.io/stefanprodan/podinfo-deploy:latest
```

## Download the artifacts with crane

Install the [crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane) CLI:

```sh
brew install crane
```

Download the podinfo deployment manifests with crane CLI:

```console
$ crane export ghcr.io/stefanprodan/podinfo-deploy:latest -| tar -xf - 

$ ls -1
deployment.yaml
hpa.yaml
kustomization.yaml
service.yaml
```
