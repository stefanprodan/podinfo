// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package ui

import "embed"

// FS holds the web UI page template, its assets and the Swagger UI.
//
//go:embed index.html assets swagger
var FS embed.FS
