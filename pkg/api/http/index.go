package http

import (
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/stefanprodan/podinfo/ui"
)

type uiPage struct {
	tmpl   *template.Template
	csp    string
	assets fs.FS
}

func newUIPage(config *Config) (*uiPage, error) {
	assets, err := fs.Sub(ui.FS, "assets")
	if err != nil {
		return nil, err
	}

	if config.UIPath != "" {
		// vue.html is the page name of the 6.x UI.
		name := filepath.Join(config.UIPath, "index.html")
		if _, err := os.Stat(name); errors.Is(err, fs.ErrNotExist) {
			name = filepath.Join(config.UIPath, "vue.html")
		}
		tmpl, err := template.ParseFiles(name)
		if err != nil {
			return nil, err
		}
		return &uiPage{tmpl: tmpl, assets: assets}, nil
	}

	tmpl, err := template.ParseFS(ui.FS, "index.html")
	if err != nil {
		return nil, err
	}
	return &uiPage{tmpl: tmpl, csp: contentSecurityPolicy(config.UILogo), assets: assets}, nil
}

func contentSecurityPolicy(logo string) string {
	img := "'self'"
	if u, err := url.Parse(logo); err == nil && u.Host != "" {
		if u.Scheme != "" {
			img += " " + u.Scheme + "://" + u.Host
		} else {
			img += " " + u.Host
		}
	}
	return "default-src 'none'; script-src 'self'; style-src 'self'; img-src " + img +
		"; connect-src 'self'; base-uri 'none'; form-action 'none'"
}

// Index godoc
// @Summary Index
// @Description renders podinfo UI
// @Tags HTTP API
// @Produce html
// @Router / [get]
// @Success 200 {string} string "OK"
func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title  string
		Logo   string
		Prefix string
	}{
		Title:  s.config.Hostname,
		Logo:   s.config.UILogo,
		Prefix: strings.TrimSuffix(s.config.Prefix, "/"),
	}

	if s.ui.csp != "" {
		w.Header().Set("Content-Security-Policy", s.ui.csp)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.ui.tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
