// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

// Package keypair serves a TLS certificate and key from a directory and
// reloads them when they are rotated.
//
// KeyPair reads tls.crt and tls.key, the layout of a kubernetes.io/tls
// Secret volume, and watches the directory with pkg/fscache. A missing or
// invalid pair is an error at startup; after that a failed reload keeps the
// last valid pair and logs a warning. GetCertificate and
// GetClientCertificate plug into a tls.Config, so new connections use the
// rotated pair without a restart.
package keypair
