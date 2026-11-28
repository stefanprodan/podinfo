// Package ui embeds the podinfo web UI: the index.html page template and its
// static assets.
//
// pkg/api/http renders the template and serves the assets under /ui/. The
// demo directory, a fake API used by make ui-demo, is not embedded.
package ui
