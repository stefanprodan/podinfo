package docs

import _ "embed"

// Swagger holds the generated spec of the HTTP API.
//
//go:embed swagger.json
var Swagger []byte
