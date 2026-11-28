package http

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/stefanprodan/podinfo/pkg/api/http/docs"
	"github.com/stefanprodan/podinfo/ui"
)

// Swagger UI loads its icons from data URLs.
const swaggerCSP = "default-src 'none'; script-src 'self'; style-src 'self'; " +
	"img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'none'"

type swaggerDocs struct {
	spec []byte
	ui   fs.FS
}

// newSwaggerDocs returns the Swagger UI and the embedded spec with basePath set to the prefix.
func newSwaggerDocs(prefix string) (*swaggerDocs, error) {
	var spec map[string]json.RawMessage
	if err := json.Unmarshal(docs.Swagger, &spec); err != nil {
		return nil, fmt.Errorf("parsing the Swagger spec failed: %w", err)
	}

	basePath, err := json.Marshal(prefix)
	if err != nil {
		return nil, err
	}
	spec["basePath"] = basePath

	doc, err := json.MarshalIndent(spec, "", "    ")
	if err != nil {
		return nil, err
	}

	assets, err := fs.Sub(ui.FS, "swagger")
	if err != nil {
		return nil, err
	}
	return &swaggerDocs{spec: doc, ui: assets}, nil
}

func (s *Server) swaggerHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(s.swagger.spec)
}

func (s *Server) swaggerUIHandler() http.Handler {
	files := http.StripPrefix(s.prefixedPath("/swagger"), http.FileServerFS(s.swagger.ui))
	index := s.prefixedPath("/swagger/index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The file server redirects index.html to the directory, which loops in
		// browsers that cached the permanent redirect to index.html of podinfo 6.
		if r.URL.Path == index {
			r = r.Clone(r.Context())
			r.URL.Path = strings.TrimSuffix(index, "index.html")
		}
		w.Header().Set("Content-Security-Policy", swaggerCSP)
		files.ServeHTTP(w, r)
	})
}
