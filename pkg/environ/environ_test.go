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
		{"url user only", "PODINFO_BACKEND_URL=https://u@backend/api", "PODINFO_BACKEND_URL=https://[REDACTED]@backend/api"},
		{"url user only token", "GIT_URL=https://ghp_abcdefTOKEN@github.com/org/repo.git", "GIT_URL=https://[REDACTED]@github.com/org/repo.git"},
		{"url user only with port", "GIT_URL=https://ghp_x@git.local:8443/org/repo.git", "GIT_URL=https://[REDACTED]@git.local:8443/org/repo.git"},
		{"url user with at sign", "SMTP_URL=smtp://me@example.com:s3cret@mail:587", "SMTP_URL=smtp://me@example.com:[REDACTED]@mail:587"},
		{"url password with scheme", "REDIS_URL=redis://user:ab://cd@host:6379", "REDIS_URL=redis://user:[REDACTED]@host:6379"},
		{"url password with digits and scheme", "REDIS_URL=redis://user:12ab://cd@host:6379", "REDIS_URL=redis://user:[REDACTED]@host:6379"},
		{"url password with space", "PG=postgres://user:pa ss@host/db", "PG=postgres://user:[REDACTED]@host/db"},
		{"url list with ports", "NODES=redis://h1:6379,redis://b:q@h2:6379", "NODES=redis://h1:6379,redis://b:[REDACTED]@h2:6379"},
		{"url list with paths", "NODES=http://h1:80/a,http://b:q@h2/b", "NODES=http://h1:80/a,http://b:[REDACTED]@h2/b"},
		{"url port then email", "ARGS=--url=http://backend:9898 --email=a@b.com", "ARGS=--url=http://backend:9898 --email=a@b.com"},
		{"url IPv6 then email", "ARGS=--url=http://[::1]:9898/echo --email=a@b.com", "ARGS=--url=http://[::1]:9898/echo --email=a@b.com"},
		{"url path with at sign", "PODINFO_BACKEND_URL=http://backend/a@c?email=a@b.com", "PODINFO_BACKEND_URL=http://backend/a@c?email=a@b.com"},
		{"url no userinfo", "PODINFO_BACKEND_URL=http://backend:9898/echo", "PODINFO_BACKEND_URL=http://backend:9898/echo"},
		{"url query api key", "PODINFO_BACKEND_URL=https://user:pw@backend:9898/echo?api_key=SECRETQ#frag", "PODINFO_BACKEND_URL=https://user:[REDACTED]@backend:9898/echo?api_key=[REDACTED]#frag"},
		{"url query password", "DB_URL=postgres://db:5432/app?user=u&password=hunter2", "DB_URL=postgres://db:5432/app?user=u&password=[REDACTED]"},
		{"url query jdbc", "JDBC=jdbc:mysql://db/app?user=u&password=hunter2", "JDBC=jdbc:mysql://db/app?user=u&password=[REDACTED]"},
		{"url query pwd and sig", "BLOB_URL=https://h/c?PWD=a&sv=2024&Sig=b%2Fc", "BLOB_URL=https://h/c?PWD=[REDACTED]&sv=2024&Sig=[REDACTED]"},
		{"url query presigned", "S3_URL=https://s3/b/o?X-Amz-Expires=60&X-Amz-Signature=abc", "S3_URL=https://s3/b/o?X-Amz-Expires=60&X-Amz-Signature=[REDACTED]"},
		{"url query token with scheme", "CALLBACK=https://h/cb?token=ab://cd@x&state=1", "CALLBACK=https://h/cb?token=[REDACTED]&state=1"},
		{"url query nested", "LOGIN=https://h/login?next=https://u:p@x/y&token=abc", "LOGIN=https://h/login?next=https://u:[REDACTED]@x/y&token=[REDACTED]"},
		{"url query harmless", "DOCS_URL=https://example.com/list?page=2&sort=name", "DOCS_URL=https://example.com/list?page=2&sort=name"},
		{"url query design", "DOCS_URL=https://example.com/list?design=flat&sigma=2", "DOCS_URL=https://example.com/list?design=flat&sigma=2"},
		{"pwd suffix", "MYSQL_PWD=hunter2", "MYSQL_PWD=[REDACTED]"},
		{"pwd suffix short", "DB_PWD=hunter2", "DB_PWD=[REDACTED]"},
		{"cookie", "RABBITMQ_ERLANG_COOKIE=abcdef", "RABBITMQ_ERLANG_COOKIE=[REDACTED]"},
		{"jwt", "JWT=eyJhbGciOi.eyJzdWIi.c2ln", "JWT=[REDACTED]"},
		{"dsn", "DATABASE_DSN=user:hunter2@tcp(db:3306)/app", "DATABASE_DSN=[REDACTED]"},
		{"sentry dsn", "SENTRY_DSN=https://abc123@o1.ingest.sentry.io/42", "SENTRY_DSN=[REDACTED]"},
		{"signature", "WEBHOOK_SIGNATURE=abc", "WEBHOOK_SIGNATURE=[REDACTED]"},
		{"assertion", "SAML_ASSERTION=abc", "SAML_ASSERTION=[REDACTED]"},
		{"key value password", "DB_CONN=host=db user=u password=hunter2", "DB_CONN=host=db user=u password=[REDACTED]"},
		{"key value password with semicolons", "CONN=Server=db;User Id=u;Password=hunter2;", "CONN=Server=db;User Id=u;Password=[REDACTED];"},
		{"key value pwd", "CONN=Server=db;Uid=u;Pwd=hunter2;Encrypt=yes", "CONN=Server=db;Uid=u;Pwd=[REDACTED];Encrypt=yes"},
		{"key value passwd", "OPTS=user=u&passwd=hunter2&ssl=on", "OPTS=user=u&passwd=[REDACTED]&ssl=on"},
		{"key value quoted password", "DB_CONN=host=db password='hun ter2' user=u", "DB_CONN=host=db password=[REDACTED] user=u"},
		{"shell pwd", "PWD=/home/app", "PWD=/home/app"},
		{"shell oldpwd", "OLDPWD=/home", "OLDPWD=/home"},
		{"hostname", "HOSTNAME=podinfo-7d8b9c-x2x4z", "HOSTNAME=podinfo-7d8b9c-x2x4z"},
		{"home", "HOME=/home/app", "HOME=/home/app"},
		{"kubernetes service host", "KUBERNETES_SERVICE_HOST=10.96.0.1", "KUBERNETES_SERVICE_HOST=10.96.0.1"},
		{"kubernetes port", "KUBERNETES_PORT=tcp://10.96.0.1:443", "KUBERNETES_PORT=tcp://10.96.0.1:443"},
		{"kubernetes port address", "KUBERNETES_PORT_443_TCP_ADDR=10.96.0.1", "KUBERNETES_PORT_443_TCP_ADDR=10.96.0.1"},
		{"podinfo color", "PODINFO_UI_COLOR=#34577c", "PODINFO_UI_COLOR=#34577c"},
		{"podinfo port", "PODINFO_PORT=9898", "PODINFO_PORT=9898"},
		{"max connections", "MAX_CONNECTIONS=100", "MAX_CONNECTIONS=100"},
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

func TestRedactedMap(t *testing.T) {
	t.Setenv("PODINFO_TEST_PLAIN", "a=b")
	t.Setenv("PODINFO_TEST_PASSWORD", "s3cret")

	env := RedactedMap()
	if got := env["PODINFO_TEST_PLAIN"]; got != "a=b" {
		t.Errorf("PODINFO_TEST_PLAIN = %q, want %q", got, "a=b")
	}
	if got := env["PODINFO_TEST_PASSWORD"]; got != redacted {
		t.Errorf("PODINFO_TEST_PASSWORD = %q, want %q", got, redacted)
	}
}

func TestRedactHeaders(t *testing.T) {
	in := map[string][]string{
		"Accept":                   {"*/*"},
		"Content-Type":             {"application/json"},
		"Authorization":            {"Bearer abc"},
		"Proxy-Authorization":      {"Basic abc"},
		"Cookie":                   {"session=abc"},
		"Set-Cookie":               {"session=abc"},
		"X-Api-Key":                {"abc"},
		"X-Auth-Request-User":      {"stefan"},
		"Cf-Access-Jwt-Assertion":  {"eyJ.a.b"},
		"X-Goog-Iap-Jwt-Assertion": {"eyJ.a.b"},
		"X-Amzn-Oidc-Data":         {"eyJ.a.b"},
		"X-Amzn-Oidc-Accesstoken":  {"eyJ.a.b"},
		"X-Amzn-Oidc-Identity":     {"stefan"},
		"x-amzn-oidc-data":         {"eyJ.a.b"},
		"X-Amz-Signature":          {"abc"},
		"X-Amzn-Trace-Id":          {"Root=1-67891233-abcdef012345678912345678"},
		"Referer":                  {"https://u:p@example.com/"},
		"Origin":                   {"https://example.com/list?page=2&sort=name"},
		"X-Original-Url":           {"https://h/p?access_token=abc"},
		"X-Forwarded-Uri":          {"/callback?code=1&access_token=abc"},
		":authority":               {"podinfo:9999"},
	}
	want := map[string][]string{
		"Accept":                   {"*/*"},
		"Content-Type":             {"application/json"},
		"Authorization":            {redacted},
		"Proxy-Authorization":      {redacted},
		"Cookie":                   {redacted},
		"Set-Cookie":               {redacted},
		"X-Api-Key":                {redacted},
		"X-Auth-Request-User":      {redacted},
		"Cf-Access-Jwt-Assertion":  {redacted},
		"X-Goog-Iap-Jwt-Assertion": {redacted},
		"X-Amzn-Oidc-Data":         {redacted},
		"X-Amzn-Oidc-Accesstoken":  {redacted},
		"X-Amzn-Oidc-Identity":     {redacted},
		"x-amzn-oidc-data":         {redacted},
		"X-Amz-Signature":          {redacted},
		"X-Amzn-Trace-Id":          {"Root=1-67891233-abcdef012345678912345678"},
		"Referer":                  {"https://u:" + redacted + "@example.com/"},
		"Origin":                   {"https://example.com/list?page=2&sort=name"},
		"X-Original-Url":           {"https://h/p?access_token=" + redacted},
		"X-Forwarded-Uri":          {"/callback?code=1&access_token=" + redacted},
		":authority":               {"podinfo:9999"},
	}

	got := RedactHeaders(in)
	for name, values := range want {
		if !slices.Equal(got[name], values) {
			t.Errorf("%s = %v, want %v", name, got[name], values)
		}
	}
	if in["Authorization"][0] != "Bearer abc" {
		t.Error("RedactHeaders modified its input")
	}
}

func TestRedactHeaders_Referer(t *testing.T) {
	tests := map[string]string{
		"https://h/p?access_token=abc":              "https://h/p?access_token=" + redacted,
		"https://example.com/list?page=2&sort=name": "https://example.com/list?page=2&sort=name",
	}
	for in, want := range tests {
		got := RedactHeaders(map[string][]string{"Referer": {in}})
		if !slices.Equal(got["Referer"], []string{want}) {
			t.Errorf("Referer %q = %v, want %q", in, got["Referer"], want)
		}
	}
}
