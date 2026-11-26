package ui

import "embed"

// FS holds the web UI page template and its assets.
//
//go:embed index.html assets
var FS embed.FS
