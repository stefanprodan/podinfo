package environ

import (
	"slices"
	"testing"
)

func TestRedact(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "PATH=/usr/bin", "PATH=/usr/bin"},
		{"password", "PODINFO_CACHE_PASSWORD=s3cret", "PODINFO_CACHE_PASSWORD=[REDACTED]"},
		{"sentinel password", "PODINFO_CACHE_SENTINEL_PASSWORD=s3cret", "PODINFO_CACHE_SENTINEL_PASSWORD=[REDACTED]"},
		{"lowercase name", "db_passwd=s3cret", "db_passwd=[REDACTED]"},
		{"pass", "REDIS_PASS=s3cret", "REDIS_PASS=[REDACTED]"},
		{"secret", "APP_SECRET=s3cret", "APP_SECRET=[REDACTED]"},
		{"token", "GITHUB_TOKEN=ghp_x", "GITHUB_TOKEN=[REDACTED]"},
		{"key", "API_KEY=abc", "API_KEY=[REDACTED]"},
		{"credential", "GOOGLE_APPLICATION_CREDENTIALS=/creds.json", "GOOGLE_APPLICATION_CREDENTIALS=[REDACTED]"},
		{"auth", "BASIC_AUTH=user:pass", "BASIC_AUTH=[REDACTED]"},
		{"private", "PRIVATE_CERT=pem", "PRIVATE_CERT=[REDACTED]"},
		{"headers", "OTEL_EXPORTER_OTLP_HEADERS=x-api=abc", "OTEL_EXPORTER_OTLP_HEADERS=[REDACTED]"},
		{"empty value", "PODINFO_CACHE_PASSWORD=", "PODINFO_CACHE_PASSWORD=[REDACTED]"},
		{"url password", "PODINFO_CACHE_SERVER=redis://user:s3cret@redis:6379/0", "PODINFO_CACHE_SERVER=redis://user:[REDACTED]@redis:6379/0"},
		{"url empty user", "PODINFO_CACHE_SERVER=redis://:s3cret@redis:6379", "PODINFO_CACHE_SERVER=redis://:[REDACTED]@redis:6379"},
		{"url query", "PODINFO_BACKEND_URL=https://u:p@backend/api?x=1", "PODINFO_BACKEND_URL=https://u:[REDACTED]@backend/api?x=1"},
		{"url password with at sign", "PODINFO_CACHE_SERVER=redis://u:p@ss@redis:6379", "PODINFO_CACHE_SERVER=redis://u:[REDACTED]@redis:6379"},
		{"url password unparsable", "PODINFO_CACHE_SERVER=redis://u:p#/x@redis:6379", "PODINFO_CACHE_SERVER=redis://u:[REDACTED]@redis:6379"},
		{"url password with comma", "PODINFO_CACHE_SERVER=redis://u:p,ss@h:6379", "PODINFO_CACHE_SERVER=redis://u:[REDACTED]@h:6379"},
		{"url path with colon", "PODINFO_BACKEND_URL=http://backend/a:b@c", "PODINFO_BACKEND_URL=http://backend/a:b@c"},
		{"url list", "NODES=redis://a:p@h1,redis://b:q@h2", "NODES=redis://a:[REDACTED]@h1,redis://b:[REDACTED]@h2"},
		{"url embedded", "ARGS=--cache-server=redis://u:p@h:6379 --x", "ARGS=--cache-server=redis://u:[REDACTED]@h:6379 --x"},
		{"url user only", "PODINFO_BACKEND_URL=https://u@backend/api", "PODINFO_BACKEND_URL=https://u@backend/api"},
		{"url no userinfo", "PODINFO_BACKEND_URL=http://backend:9898/echo", "PODINFO_BACKEND_URL=http://backend:9898/echo"},
		{"value with equals", "OPTS=a=b", "OPTS=a=b"},
		{"no equals", "MALFORMED", "MALFORMED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redact([]string{tt.in})
			if got[0] != tt.want {
				t.Errorf("redact(%q) = %q, want %q", tt.in, got[0], tt.want)
			}
		})
	}
}

func TestRedactPreservesOrder(t *testing.T) {
	in := []string{"B=1", "A_TOKEN=x", "C=3"}
	want := []string{"B=1", "A_TOKEN=[REDACTED]", "C=3"}
	if got := redact(in); !slices.Equal(got, want) {
		t.Errorf("redact(%q) = %q, want %q", in, got, want)
	}
}
