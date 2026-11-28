// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

// Package fscache keeps an in-memory snapshot of the files in a directory and
// refreshes it when they change.
//
// Watcher reads the whole directory, or only the files named with WithFiles,
// and swaps the snapshot atomically, so readers never see a mix of old and
// new files. It follows the ..data symlink swap that Kubernetes uses to
// update ConfigMap and Secret volumes. Changes are debounced, and a periodic
// resync picks up edits the file system reports no event for. When a reload
// fails the previous snapshot stays in place.
//
// podinfo uses it for the /configs endpoint and, through pkg/keypair, for
// TLS key pairs.
package fscache
