// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

// Command podcli is the client that ships in the podinfo container image.
//
// The check subcommands (http, tcp, grpc, cert and ws) exit non-zero when the
// check fails, so they serve as Kubernetes exec probes and as Helm and Timoni
// test hooks; all but cert can retry with a delay. The ws subcommand opens an
// interactive WebSocket session against the /ws/echo endpoint.
package main
