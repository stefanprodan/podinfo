package http

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stefanprodan/podinfo/ui"
)

func serveIndex(t *testing.T, srv *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", target, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET %s returned %d, want %d", target, rr.Code, http.StatusOK)
	}
	return rr
}

func TestIndexHandler(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		target string
	}{
		{name: "root", prefix: "/", target: "/"},
		{name: "prefix", prefix: "/foo", target: "/foo"},
		{name: "prefix trailing slash", prefix: "/foo", target: "/foo/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewMockServer()
			srv.config.Prefix = tt.prefix
			srv.config.UILogo = "/logo.gif"
			srv.registerHandlers()

			rr := serveIndex(t, srv, tt.target)

			if got := rr.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q", got)
			}
			if rr.Header().Get("Content-Security-Policy") == "" {
				t.Error("Content-Security-Policy header is missing")
			}

			base := strings.TrimSuffix(tt.prefix, "/")
			body := rr.Body.String()
			for _, want := range []string{
				"<title>localhost</title>",
				`data-prefix="` + base + `"`,
				`src="/logo.gif"`,
				`href="` + base + `/ui/podinfo.css"`,
				`src="` + base + `/ui/podinfo.js"`,
				`href="` + base + `/swagger/"`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %s", want)
				}
			}
		})
	}
}

func TestContentSecurityPolicy(t *testing.T) {
	tests := []struct {
		logo string
		img  string
	}{
		{logo: "/ui/cuddle_clap.gif", img: "img-src 'self';"},
		{logo: "https://example.com:8443/logo.png", img: "img-src 'self' https://example.com:8443;"},
		{logo: "//example.com/logo.png", img: "img-src 'self' example.com;"},
	}

	for _, tt := range tests {
		t.Run(tt.logo, func(t *testing.T) {
			csp := contentSecurityPolicy(tt.logo)
			if !strings.Contains(csp, tt.img) {
				t.Errorf("CSP %q does not contain %q", csp, tt.img)
			}
			if !strings.HasPrefix(csp, "default-src 'none'; script-src 'self'; style-src 'self';") {
				t.Errorf("CSP %q allows more than the page's own scripts and styles", csp)
			}
		})
	}
}

func TestIndexNoExternalOrigins(t *testing.T) {
	allowed := map[string]bool{
		"https://github.com/stefanprodan/podinfo": true,
		"http://www.w3.org/2000/svg":              true,
	}
	external := regexp.MustCompile(`(?:[a-z]+:)?//[^\s"'<>()]+`)
	check := func(name, content string) {
		for _, u := range external.FindAllString(content, -1) {
			if !allowed[u] {
				t.Errorf("%s references external origin %s", name, u)
			}
		}
	}

	srv := NewMockServer()
	srv.config.UILogo = "/ui/cuddle_clap.gif"
	srv.registerHandlers()
	check("index", serveIndex(t, srv, "/").Body.String())

	err := fs.WalkDir(ui.FS, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(name) == ".gif" {
			return err
		}
		data, err := fs.ReadFile(ui.FS, name)
		if err != nil {
			return err
		}
		check(name, string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUIAssets(t *testing.T) {
	tests := map[string]string{
		"favicon.svg":      "image/svg+xml",
		"podinfo.css":      "text/css; charset=utf-8",
		"podinfo.js":       "text/javascript; charset=utf-8",
		"cuddle_clap.gif":  "image/gif",
		"cuddle_bunny.gif": "image/gif",
	}

	for _, prefix := range []string{"/", "/foo"} {
		srv := NewMockServer()
		srv.config.Prefix = prefix
		srv.registerHandlers()

		for name, contentType := range tests {
			target := strings.TrimSuffix(prefix, "/") + "/ui/" + name
			req := httptest.NewRequest("GET", target, nil)
			rr := httptest.NewRecorder()
			srv.router.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("GET %s returned %d, want %d", target, rr.Code, http.StatusOK)
				continue
			}
			if got := rr.Header().Get("Content-Type"); got != contentType {
				t.Errorf("GET %s Content-Type = %q, want %q", target, got, contentType)
			}
			if rr.Body.Len() == 0 {
				t.Errorf("GET %s returned an empty body", target)
			}
		}
	}
}

func TestIndexUIPath(t *testing.T) {
	for _, name := range []string{"index.html", "vue.html"} {
		t.Run(name, func(t *testing.T) { testIndexUIPath(t, name) })
	}

	dir := t.TempDir()
	for name, title := range map[string]string{"index.html": "new", "vue.html": "old"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("<title>"+title+"</title>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	srv := NewMockServer()
	srv.config.UIPath = dir
	page, err := newUIPage(srv.config)
	if err != nil {
		t.Fatal(err)
	}
	srv.ui = page
	srv.registerHandlers()
	if body := serveIndex(t, srv, "/").Body.String(); !strings.Contains(body, "<title>new</title>") {
		t.Errorf("body = %s, want index.html to win over vue.html", body)
	}
}

func testIndexUIPath(t *testing.T, name string) {
	dir := t.TempDir()
	custom := `<html><title>{{.Title}}</title><script src="https://cdn.example.com/app.js"></script>{{.Prefix}}</html>`
	if err := os.WriteFile(filepath.Join(dir, name), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := NewMockServer()
	srv.config.Prefix = "/foo"
	srv.config.UIPath = dir
	page, err := newUIPage(srv.config)
	if err != nil {
		t.Fatal(err)
	}
	srv.ui = page
	srv.registerHandlers()

	rr := serveIndex(t, srv, "/foo/")
	if !strings.Contains(rr.Body.String(), "<title>localhost</title>") {
		t.Errorf("body = %s", rr.Body.String())
	}
	if csp := rr.Header().Get("Content-Security-Policy"); csp != "" {
		t.Errorf("Content-Security-Policy = %q, want none for a --ui-path page", csp)
	}

	srv.config.UIPath = filepath.Join(dir, "missing")
	if _, err := newUIPage(srv.config); err == nil {
		t.Error("newUIPage succeeded for a directory without a page")
	}
}
