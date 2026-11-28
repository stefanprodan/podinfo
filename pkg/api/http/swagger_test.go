package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestSwagger(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		basePath string
	}{
		{name: "root", prefix: "/", basePath: "/"},
		{name: "empty", prefix: "", basePath: "/"},
		{name: "prefix", prefix: "/foo", basePath: "/foo"},
		{name: "prefix trailing slash", prefix: "/foo/", basePath: "/foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := NewServer(&Config{Prefix: tt.prefix}, zap.NewNop())
			if err != nil {
				t.Fatalf("NewServer returned %v", err)
			}
			srv.registerHandlers()

			base := strings.TrimSuffix(tt.basePath, "/")
			do := func(method, route string) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.router.ServeHTTP(rr, httptest.NewRequest(method, base+route, nil))
				return rr
			}
			get := func(route string) *httptest.ResponseRecorder { return do("GET", route) }

			for _, route := range []string{"/swagger.json", "/swagger/doc.json"} {
				rr := get(route)
				if rr.Code != http.StatusOK {
					t.Fatalf("GET %s returned %d, want %d", base+route, rr.Code, http.StatusOK)
				}
				if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
					t.Errorf("GET %s Content-Type = %q", base+route, got)
				}
				var spec struct {
					Swagger  string                     `json:"swagger"`
					BasePath string                     `json:"basePath"`
					Paths    map[string]json.RawMessage `json:"paths"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
					t.Fatalf("GET %s returned invalid JSON: %v", base+route, err)
				}
				if spec.Swagger != "2.0" {
					t.Errorf("GET %s swagger = %q, want 2.0", base+route, spec.Swagger)
				}
				if spec.BasePath != tt.basePath {
					t.Errorf("GET %s basePath = %q, want %q", base+route, spec.BasePath, tt.basePath)
				}
				if _, ok := spec.Paths["/api/info"]; !ok {
					t.Errorf("GET %s has no /api/info path", base+route)
				}

				if code := do("HEAD", route).Code; code != http.StatusOK {
					t.Errorf("HEAD %s returned %d, want %d", base+route, code, http.StatusOK)
				}
				if code := do("POST", route).Code; code != http.StatusMethodNotAllowed {
					t.Errorf("POST %s returned %d, want %d", base+route, code, http.StatusMethodNotAllowed)
				}
			}

			rr := get("/swagger")
			if rr.Code != http.StatusMovedPermanently {
				t.Errorf("GET %s returned %d, want %d", base+"/swagger", rr.Code, http.StatusMovedPermanently)
			}
			if got := rr.Header().Get("Location"); got != base+"/swagger/" {
				t.Errorf("GET %s Location = %q, want %q", base+"/swagger", got, base+"/swagger/")
			}

			for _, page := range []string{"/swagger/", "/swagger/index.html"} {
				rr := get(page)
				if rr.Code != http.StatusOK {
					t.Fatalf("GET %s returned %d, want %d", base+page, rr.Code, http.StatusOK)
				}
				if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
					t.Errorf("GET %s Content-Type = %q", base+page, got)
				}
				if got := rr.Header().Get("Content-Security-Policy"); got != swaggerCSP {
					t.Errorf("GET %s Content-Security-Policy = %q", base+page, got)
				}
				if !strings.Contains(rr.Body.String(), `src="dist/swagger-ui-bundle.js"`) {
					t.Errorf("GET %s does not load the Swagger UI bundle", base+page)
				}
			}

			for _, asset := range []string{"dist/swagger-ui-bundle.js", "dist/swagger-ui.css", "swagger.js", "swagger.css"} {
				if code := get("/swagger/" + asset).Code; code != http.StatusOK {
					t.Errorf("GET %s returned %d, want %d", base+"/swagger/"+asset, code, http.StatusOK)
				}
			}
		})
	}
}
