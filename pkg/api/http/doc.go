// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

// Package http implements the podinfo HTTP server.
//
// Server routes the API with gorilla/mux: pod information (/api/info,
// /api/pod, /version, /env, /headers), probes (/healthz, /readyz and the
// readiness toggles), fault and latency helpers (/status/{code},
// /delay/{seconds}, /chunked, /panic, random delay and random error
// middleware), fan-out to backends (/echo, /ws/echo), a Redis or Valkey
// cache (/cache/{key}), a file store (/store), JWT issuing and validation
// (/token), the embedded web UI, Swagger docs, Prometheus metrics and pprof.
//
// The same handler is served on the HTTP port and, when --secure-port is
// set, over TLS with the key pair from --cert-path or with SPIFFE mTLS.
// --port-metrics serves /metrics, /healthz and pprof on a port of their own.
// Requests are traced with otelhttp and measured with Prometheus histograms
// that carry trace exemplars.
package http
