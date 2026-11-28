// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

// Package grpc implements the podinfo gRPC server.
//
// Server registers the services generated under the v1 subpackages (Info,
// Version, Pod, Env, Header, Echo, Delay, Status, Panic, Token, Cache and
// Store), which mirror the HTTP API in pkg/api/http and share its limits,
// store and credential masking. It also registers the gRPC health service
// and server reflection.
//
// The port is plaintext by default. With --grpc-tls it serves TLS with the
// key pair from --cert-path, or SPIFFE mTLS when a Workload API socket is
// configured. Echo forwards to the --grpc-backend-url backends. Interceptors
// recover from panics and record Prometheus metrics and OpenTelemetry
// traces.
package grpc
