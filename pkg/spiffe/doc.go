// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

// Package spiffe shares an X.509 SVID from the SPIFFE Workload API between
// the podinfo servers.
//
// Source wraps one Workload API client that the HTTPS and gRPC servers both
// use, so they present the same identity and rotate it together. Until the
// first SVID is issued it returns errors, and Wait blocks until then. The
// server TLS configuration requires a client SVID and authorizes it against
// a list of SPIFFE IDs or, when the list is empty, against podinfo's own
// trust domain; the client configuration does the same for backends.
package spiffe
