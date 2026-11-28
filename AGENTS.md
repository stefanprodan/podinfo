# AGENTS instructions

## Project Overview

Podinfo is a tiny web application made with Go that showcases best practices of running microservices in Kubernetes.
It is used by CNCF projects like Flux and Flagger for end-to-end testing and workshops.

The project is structured as a Go module with the following components:

* **podinfo**: The server. It exposes an HTTP API built with gorilla/mux, a gRPC API, a web UI and Prometheus metrics, and exports traces, metrics and logs with OpenTelemetry. It is configured with viper from flags, `PODINFO_` environment variables and a config file.
* **podcli**: A command-line interface with health checks for HTTP, TCP, TLS certificates, WebSocket and gRPC endpoints. It is built using the Cobra library.
* **Web UI**: Plain HTML, CSS and JavaScript embedded in the binary. It has no build step and makes no requests to other origins.
* **Installers**: A Helm chart, a Timoni module and a Kustomize overlay that deploy podinfo on Kubernetes.

### Repository Structure

```bash
├── charts/         # Helm chart
├── cmd/            # Main entrypoint for the binaries
│   ├── podcli/     # podinfo CLI
│   └── podinfo/    # podinfo server
├── deploy/         # Kustomize demo with frontend, backend and cache
├── docs/           # API reference and guides
├── kustomize/      # Kustomize overlay
├── otel/           # Docker Compose setup for OpenTelemetry
├── pkg/            # Go packages
│   ├── api/grpc/   # gRPC services, protos and generated stubs
│   ├── api/http/   # HTTP handlers, middlewares and the Swagger spec
│   ├── signals/    # Signal handling and graceful shutdown
│   ├── telemetry/  # OpenTelemetry setup and log correlation
│   └── version/    # Release version and Git revision
├── test/           # End-to-end tests with Kubernetes Kind
├── timoni/         # Timoni module
└── ui/             # Web UI and the vendored Swagger UI
```

## Rules of Engagement for AI Agents

- Do not deviate from the established patterns in the codebase
- New Go and JavaScript files must have the license header with the current year, never change the year of an existing header
- All new features must have associated documentation and tests
- Never edit generated or vendored files by hand: `*.pb.go`, `pkg/api/http/docs/swagger.json`, `charts/podinfo/values.schema.json` and `ui/swagger/dist`
- Exported Go identifiers must have a doc comment that says what the name and signature do not
- After modifying a function or type, update its doc comment
- Add comments for complex logic but don't comment obvious code
- Every HTTP handler must have Swagger annotations that match its methods, content types and status codes
- A `//nolint` directive must name the linter and give the reason, never use `golangci-lint run --fix`
- The web UI must not load scripts, styles, fonts or images from other origins
- Keep the Helm chart, the Timoni module and the Kustomize overlay in sync when changing how podinfo is deployed
- Name new make targets verb first and give them a `##` description
- Read existing tests before writing new ones
- Run individual tests when debugging, run the full test suite when done
- Use `go doc` to read func signatures in external packages

## Development Commands for AI Agents

- Run `make help` to list the make targets
- Run `make gen` after modifying protos, Swagger annotations or the Helm chart `values.yaml`
- Run `make fmt lint` after completing a task and fix lint errors
- Run `make test` to execute all Go tests
- Run `make run` to start podinfo locally on port 9898, with gRPC on port 9999
- Run `make run-ui-demo` to review the web UI against a fake API
- Run `make build-chart` and `make build-timoni` after modifying the Helm chart or the Timoni module
- Follow the [maintenance guide](docs/maintenance.md) to update pinned versions and to cut a release

## Commits and Pull Requests for AI Agents

- Work on a branch, never commit to `master`
- Write the commit subject in the imperative, capitalised, with no type prefix and no trailing period, like `Add license header`
- Leave out the commit body unless the reason for the change is not obvious, then keep it to a line or two
- Sign off every commit with `git commit -s`, the `Signed-off-by` trailer must name the human author
- Do not add `Co-authored-by` trailers or session links for AI agents
- Disclose the AI assistance with an `Assisted-by: <agent>/<model>` commit trailer and in the PR description
- Show the PR title and body to the user and wait for approval before opening or editing a PR
- Write the PR body as a short list of what changed, and list the behaviour changes separately
