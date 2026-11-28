package ui

import "embed"

// FS holds the web UI page template, its assets and the Swagger UI.
//
//go:embed index.html assets swagger
var FS embed.FS
