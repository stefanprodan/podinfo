# Podinfo Maintenance

How to update the versions pinned in this repository and how to cut a release.

Make each update in its own pull request, unless a section says to combine two.
Before pushing, run `make test lint gen` and commit everything it changes.
CI fails if the tree is dirty after `make test` or `make gen`.

## What is pinned where

Workflow file names refer to `.github/workflows`.

| What | Where |
|------|-------|
| Go | `go.mod`, `Dockerfile`, `test.yml`, `release.yml` |
| Go modules | `go.mod` |
| Alpine | `Dockerfile` |
| swag | `go.mod` tool directive |
| protoc-gen-go-grpc | `buf.gen.yaml` |
| Helm chart values schema generator | `Makefile`, `gen-chart-schema` target |
| Swagger UI | `Makefile`, `SWAGGER_UI_VERSION` and `SWAGGER_UI_SHA512` |
| golangci-lint | `.golangci-lint-version` |
| Biome | `biome.json`, `test.yml` |
| buf | `test.yml` |
| Helm | `test.yml`, `e2e.yml`, `release.yml` |
| Notation | `release.yml` |
| GitHub Actions | `.github/workflows` (Dependabot) |
| Redis image | `charts/podinfo/values.yaml`, `deploy/bases/cache/deployment.yaml` |
| e2e charts and images | `test/*.sh`, `e2e.yml` |
| Grafana LGTM image | `otel/docker-compose.yaml` |
| Timoni vendored schemas | `timoni/podinfo/cue.mod` |

Not pinned: goreleaser and the Flux CLI run at their latest release, and govulncheck runs on
the stable Go. The Kubernetes versions in the READMEs and `kubeVersion` in `Chart.yaml` are
support floors and are not bumped by this guide.

## Go

The Go minor version is set in four places that must agree:

- the `go` line of `go.mod`
- the `golang:<version>-alpine` builder image in `Dockerfile`
- `go-version` in `test.yml` and in `release.yml`

golangci-lint refuses to run when it was built with an older Go than the `go` line,
so update golangci-lint in the same pull request when the minor version changes.
Delete `bin/golangci-lint-*` after changing the local Go version, `make lint-go` rebuilds it.

## Go modules

```sh
go get -u ./...
make tidy
```

`make tidy` deletes `go.sum` and rebuilds it. Replace the deprecated calls the update brings in,
`make lint-go` reports them. If `go get` raises the `go` line of `go.mod`, the Go section applies.
The command does not update swag, see Code generators.

If the `cve-scan` check fails on the pull request, update the module govulncheck names.

## Alpine

Set the `FROM alpine:<version>` line of `Dockerfile` when a new stable branch is released.
The tag has no patch number, so image builds follow the patch releases without a change here.
Run `make build-container`. It fails if `apk` cannot install `ca-certificates`, `curl` or `netcat-openbsd`.

## Code generators

swag generates the Swagger spec and is a tool dependency in `go.mod`:

```sh
go get -tool github.com/swaggo/swag/cmd/swag@<version>
make tidy gen-swagger
```

Stay on swag v1. The spec is Swagger 2.0 on purpose: `/swagger.json` is documented as the
input for Linkerd service profiles and Gloo route discovery.

`protoc-gen-go` runs at the version of `google.golang.org/protobuf` in `go.mod`.
`protoc-gen-go-grpc` is pinned in `buf.gen.yaml`. After changing either, run `make gen-proto`.

The Helm chart values schema generator is pinned in the `gen-chart-schema` target.
After changing it, run `make gen-chart-schema`.

Commit the regenerated files in the same pull request, CI fails on any difference after `make gen`.

## Swagger UI

The bundle and stylesheet in `ui/swagger/dist` come from the `swagger-ui-dist` npm package.
Set `SWAGGER_UI_VERSION` in the `Makefile`, then set `SWAGGER_UI_SHA512` to the checksum
npm publishes for that version:

```sh
curl -s https://registry.npmjs.org/swagger-ui-dist/<version> \
  | jq -r .dist.integrity | cut -d- -f2 | base64 -d | xxd -p -c 256
```

1. Run `make vendor-swagger-ui` and commit `ui/swagger/dist`.
2. Run `make run` and open `http://localhost:9898/swagger/`. The page must load with no
   Content-Security-Policy errors in the console, and "Try it out" must work on an endpoint.
3. Repeat with `make run EXTRA_RUN_ARGS="--prefix /foo"` at `http://localhost:9898/foo/swagger/`.

The page is served with `style-src 'self'`. If a new Swagger UI version needs inline styles,
relax `swaggerCSP` in `pkg/api/http/swagger.go` rather than loading anything from another origin.

## Linters

Set the golangci-lint version in `.golangci-lint-version`. The Makefile and the golangci-lint
action in `test.yml` both read it. Run `make lint-go` and fix what the new version reports.
Do not use `golangci-lint run --fix`, its fixers have produced wrong code in this repository.

Set the Biome version in the `$schema` URL of `biome.json` and in `test.yml`.
`biome --version` must print the same version locally. Run `make lint-ui`.

Set the buf version in `test.yml`. `buf --version` must print the same version locally.
Run `make lint-proto gen-proto`.

## Helm

The Helm version is `HELM_VERSION` in `test.yml` and a literal in `e2e.yml` (three jobs) and
`release.yml`. Change all five. The literals carry a `v` prefix, `HELM_VERSION` does not.
Run `make build-chart`.

## GitHub Actions

Dependabot opens one grouped pull request a week for the actions, configured in
`.github/dependabot.yaml`. Actions are pinned by commit SHA with the version in a comment.
Keep that form when adding one.

The Timoni CLI, Kind and Cosign versions are the defaults of their setup actions, so they
change with the Dependabot pull request. The Notation CLI version is an input of its setup
action in `release.yml` and is set by hand.

## Images and charts used by the manifests and tests

- Redis: `redis.tag` in `charts/podinfo/values.yaml` and the image in
  `deploy/bases/cache/deployment.yaml`.
- cert-manager and Valkey charts: variables at the top of `test/e2e.sh`.
- SPIRE charts and grpcurl: variables at the top of `test/e2e-spiffe.sh`. The SPIRE agent
  image is a literal further down in the same file.
- OpenTelemetry Collector and grpcurl: variables at the top of `test/e2e-otel.sh`.
- Registry image: the `registry` service of the `kind-timoni` job in `e2e.yml`.
- Grafana LGTM: `otel/docker-compose.yaml`. Check it with the steps in `otel/README.md`,
  nothing in CI runs that stack.

The `e2e` workflow tests the rest on Kubernetes Kind: `kind-helm` runs `test/e2e.sh` with the
chart's Redis over TLS, `kind-spiffe` runs `test/e2e-spiffe.sh`, `kind-otel` runs
`test/e2e-otel.sh`, and `kind-timoni` applies `deploy/bases/cache`.

## Timoni vendored schemas

The schemas under `timoni/podinfo/cue.mod` are vendored. Each generated file records the
command that produced it in a `//timoni:generate` header. Run them from `timoni/podinfo`:

```sh
timoni mod vendor k8s -v <kubernetes minor version>
timoni mod vendor crd -f https://github.com/kubernetes-sigs/gateway-api/releases/download/<version>/standard-install.yaml --kind HTTPRoute --version v1
timoni mod vendor crd -f https://github.com/kubernetes-sigs/gateway-api/releases/download/<version>/standard-install.yaml --kind GRPCRoute --version v1
timoni mod vendor crd -f https://github.com/prometheus-operator/prometheus-operator/releases/latest/download/stripped-down-crds.yaml --kind ServiceMonitor --version v1
timoni artifact pull oci://ghcr.io/stefanprodan/timoni/schemas -o cue.mod/pkg
```

Without `--kind`, the `crd` command vendors every kind in the file. The last command updates
the Timoni core schemas in `cue.mod/pkg/timoni.sh`. The CUE language version is in
`cue.mod/module.cue`.

From `timoni/podinfo`, run `timoni mod vet .` and `timoni fmt --diff`, as CI does.
From the repository root, run `make build-timoni`.

## Release

The version is in `pkg/version/version.go`. A release is a pull request that bumps it,
followed by a signed tag on `master`.

1. On an up-to-date `master`, run `make prep-release` for the next minor version, or
   `make prep-release TAG=<version>` for a specific one. It creates the `release-<version>`
   branch, runs `make set-version`, commits `Release <version>`, pushes and opens the pull request.
2. Merge the pull request when the checks pass.
3. Run `git checkout master && git pull`, then `make release`. It creates a signed tag named
   after the version, without a `v` prefix, and pushes it.

`make set-version TAG=<version>` rewrites the version in `pkg/version/version.go`,
`charts/podinfo/Chart.yaml` and `values.yaml`, `timoni/podinfo/images.cue` and the
`podinfo:<version>` image references under `deploy` and `kustomize`. It fails if no manifest
contains the current version.

The tag starts the `release` workflow, which publishes:

- the multi-arch container image to Docker Hub and GHCR, tagged with the version and `latest`,
  with its SBOM and provenance
- the Timoni module and the Helm chart to GHCR, and the chart to the GitHub Pages Helm repository
- the Flux artifacts `ghcr.io/stefanprodan/manifests/podinfo` and `ghcr.io/stefanprodan/podinfo-deploy`
- Cosign signatures for all of them, and Notation signatures for the GHCR image, the chart
  and the Flux artifacts
- the GitHub release with the `podcli` binaries for Linux, macOS and Windows on amd64, and an
  attestation of their checksums

GitHub generates the release notes from the merged pull requests. Highlights, breaking changes
and upgrade steps are added to the release by hand after it is published.
