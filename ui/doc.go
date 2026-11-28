// Package ui embeds the podinfo web UI: the index.html page template, its
// static assets and the Swagger UI.
//
// pkg/api/http renders the template, serves the assets under /ui/ and the
// swagger directory under /swagger/. The Swagger UI bundle and stylesheet in
// swagger/dist are vendored by make swagger-ui. The demo directory, a fake
// API used by make ui-demo, is not embedded.
package ui
