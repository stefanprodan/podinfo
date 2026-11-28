// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

// Command podinfo is a small web service for testing and demoing Kubernetes
// deployments, service meshes and delivery tooling.
//
// It serves an HTTP API with a web UI, and optionally an HTTPS port, a gRPC
// API and a separate metrics port. The servers are implemented in
// pkg/api/http and pkg/api/grpc; this command parses the configuration,
// wires them together and runs them until SIGINT or SIGTERM.
//
// Configuration comes from flags, from PODINFO_* environment variables
// (--grpc-port is PODINFO_GRPC_PORT) and from an optional config file under
// --config-path. OpenTelemetry is configured with the standard OTEL_*
// variables.
package main
