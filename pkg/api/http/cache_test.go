package http

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace"

	"github.com/stefanprodan/podinfo/pkg/version"
)

func TestCacheWriteHandler_NoCache(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/cache/{key}", srv.cacheWriteHandler)

	req, _ := http.NewRequest("POST", "/cache/key1", strings.NewReader("value1"))
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server is offline") {
		t.Errorf("expected offline error, got: %s", rr.Body.String())
	}
}

func TestCacheDeleteHandler_NoCache(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/cache/{key}", srv.cacheDeleteHandler).Methods("DELETE")

	req, _ := http.NewRequest("DELETE", "/cache/key1", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server is offline") {
		t.Errorf("expected offline error, got: %s", rr.Body.String())
	}
}

func TestCacheReadHandler_NoCache(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/cache/{key}", srv.cacheReadHandler).Methods("GET")

	req, _ := http.NewRequest("GET", "/cache/key1", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server is offline") {
		t.Errorf("expected offline error, got: %s", rr.Body.String())
	}
}

func TestCacheHandlers(t *testing.T) {
	for _, cluster := range []bool{false, true} {
		t.Run(map[bool]string{false: "standalone", true: "cluster"}[cluster], func(t *testing.T) {
			m := miniredis.RunT(t)
			m.RequireUserAuth("podinfo", "secret")
			srv := newCacheMockServer(t, &Config{
				CacheServer:   "valkey://podinfo@" + m.Addr(),
				CacheCluster:  cluster,
				CachePassword: "secret",
			})

			if rr := serveCache(srv, "POST", "value1"); rr.Code != http.StatusAccepted {
				t.Fatalf("POST returned %d, want %d", rr.Code, http.StatusAccepted)
			}
			if got, _ := m.Get("key1"); got != "value1" {
				t.Fatalf("cache has %q, want %q", got, "value1")
			}
			if rr := serveCache(srv, "GET", ""); rr.Code != http.StatusOK || rr.Body.String() != "value1" {
				t.Fatalf("GET returned %d %q, want %d %q", rr.Code, rr.Body.String(), http.StatusOK, "value1")
			}
			if rr := serveCache(srv, "DELETE", ""); rr.Code != http.StatusAccepted {
				t.Fatalf("DELETE returned %d, want %d", rr.Code, http.StatusAccepted)
			}
			if rr := serveCache(srv, "GET", ""); rr.Code != http.StatusNotFound {
				t.Fatalf("GET after DELETE returned %d, want %d", rr.Code, http.StatusNotFound)
			}
		})
	}
}

func TestCacheHandlers_Error(t *testing.T) {
	for _, tt := range []struct{ method, message string }{
		{method: "POST", message: "cache set failed"},
		{method: "GET", message: "cache get failed"},
		{method: "DELETE", message: "cache delete failed"},
	} {
		t.Run(tt.method, func(t *testing.T) {
			m := miniredis.RunT(t)
			srv := newCacheMockServer(t, &Config{CacheServer: "redis://" + m.Addr()})
			m.SetError("LOADING")

			rr := serveCache(srv, tt.method, "value1")
			if rr.Code != http.StatusInternalServerError {
				t.Fatalf("%s returned %d, want %d", tt.method, rr.Code, http.StatusInternalServerError)
			}
			var resp ErrorMessage
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || resp.Message != tt.message {
				t.Errorf("%s returned %q, want message %q", tt.method, rr.Body.String(), tt.message)
			}
		})
	}
}

func TestCacheTLS(t *testing.T) {
	ts := httptest.NewTLSServer(nil)
	cert := ts.TLS.Certificates[0]
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	ts.Close()

	m := miniredis.NewMiniRedis()
	if err := m.StartTLS(&tls.Config{Certificates: []tls.Certificate{cert}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)

	srv := newCacheMockServer(t, &Config{CacheServer: "valkeys://" + m.Addr(), CacheCAFile: caFile})
	if rr := serveCache(srv, "POST", "value1"); rr.Code != http.StatusAccepted {
		t.Fatalf("POST over TLS returned %d, want %d", rr.Code, http.StatusAccepted)
	}

	srv = newCacheMockServer(t, &Config{CacheServer: "rediss://" + m.Addr()})
	if rr := serveCache(srv, "POST", "value1"); !strings.Contains(rr.Body.String(), "cache set failed") {
		t.Fatalf("POST without the CA returned %q, want a cache set error", rr.Body.String())
	}
}

func TestCacheVersion(t *testing.T) {
	m := miniredis.RunT(t)
	srv := newCacheMockServer(t, &Config{CacheServer: "redis://" + m.Addr(), Hostname: "podinfo-0"})

	srv.startCacheVersion(time.Hour)
	close(srv.cacheStop)
	<-srv.cacheStopped

	if got, _ := m.Get("podinfo-0"); got != version.VERSION {
		t.Errorf("cache has version %q, want %q", got, version.VERSION)
	}
	if ttl := m.TTL("podinfo-0"); ttl != time.Minute {
		t.Errorf("version TTL is %v, want %v", ttl, time.Minute)
	}
}

func TestCloseReleasesCache(t *testing.T) {
	m := miniredis.RunT(t)
	srv := newCacheMockServer(t, &Config{CacheServer: "tcp://" + m.Addr()})
	srv.startCacheVersion(time.Hour)

	srv.Stop()

	if rr := serveCache(srv, "POST", "value1"); rr.Code != http.StatusAccepted {
		t.Fatalf("cache write after Stop returned %d, want %d", rr.Code, http.StatusAccepted)
	}

	if err := srv.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}
	if err := srv.cache.Ping(t.Context()).Err(); err != redis.ErrClosed {
		t.Errorf("cache client returned %v after Close, want %v", err, redis.ErrClosed)
	}
	select {
	case <-srv.cacheStopped:
	default:
		t.Error("cache version goroutine is running after Close")
	}
}

func TestNewCacheOptions(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   redis.UniversalOptions
		tls    bool
	}{
		{
			name:   "tcp",
			config: Config{CacheServer: "tcp://user:pass@cache:6380"},
			want:   redis.UniversalOptions{Addrs: []string{"cache:6380"}, Username: "user", Password: "pass"},
		},
		{
			name:   "redis default port and database",
			config: Config{CacheServer: "redis://cache/2"},
			want:   redis.UniversalOptions{Addrs: []string{"cache:6379"}, DB: 2},
		},
		{
			name:   "rediss",
			config: Config{CacheServer: "rediss://:pass@cache:6380"},
			want:   redis.UniversalOptions{Addrs: []string{"cache:6380"}, Password: "pass"},
			tls:    true,
		},
		{
			name:   "valkey",
			config: Config{CacheServer: "valkey://cache:6379"},
			want:   redis.UniversalOptions{Addrs: []string{"cache:6379"}},
		},
		{
			name:   "valkeys",
			config: Config{CacheServer: "valkeys://cache:6379"},
			want:   redis.UniversalOptions{Addrs: []string{"cache:6379"}},
			tls:    true,
		},
		{
			name:   "env credentials win over the URL",
			config: Config{CacheServer: "redis://user:pass@cache:6379", CacheUsername: "env-user", CachePassword: "env-pass"},
			want:   redis.UniversalOptions{Addrs: []string{"cache:6379"}, Username: "env-user", Password: "env-pass"},
		},
		{
			name:   "cluster seed nodes",
			config: Config{CacheServer: "redis://node-0:6379?addr=node-1:6379&addr=node-2:6379"},
			want:   redis.UniversalOptions{Addrs: []string{"node-0:6379", "node-1:6379", "node-2:6379"}},
		},
		{
			name:   "cluster configuration endpoint",
			config: Config{CacheServer: "rediss://cluster.cache:6379", CacheCluster: true},
			want:   redis.UniversalOptions{Addrs: []string{"cluster.cache:6379"}, IsClusterMode: true},
			tls:    true,
		},
		{
			name: "sentinel",
			config: Config{
				CacheServer:           "redis://sentinel-0:26379/1?addr=sentinel-1:26379",
				CacheSentinelPrimary:  "primary",
				CachePassword:         "pass",
				CacheSentinelPassword: "sentinel-pass",
			},
			want: redis.UniversalOptions{
				Addrs:            []string{"sentinel-0:26379", "sentinel-1:26379"},
				DB:               1,
				MasterName:       "primary",
				Password:         "pass",
				SentinelPassword: "sentinel-pass",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newCacheOptions(&tt.config)
			if err != nil {
				t.Fatalf("newCacheOptions returned %v", err)
			}
			if (got.TLSConfig != nil) != tt.tls {
				t.Errorf("TLS is %v, want %v", got.TLSConfig != nil, tt.tls)
			}
			got.TLSConfig = nil
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("newCacheOptions returned %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestNewCacheOptions_Disabled(t *testing.T) {
	opts, err := newCacheOptions(&Config{})
	if opts != nil || err != nil {
		t.Errorf("newCacheOptions returned %v, %v, want nil, nil", opts, err)
	}
}

func TestNewCacheOptions_Invalid(t *testing.T) {
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := map[string]Config{
		"bad URL":                     {CacheServer: "redis://user:secret@cache:port"},
		"unknown scheme":              {CacheServer: "memcached://cache:11211"},
		"no host":                     {CacheServer: "redis://:6379"},
		"bare address":                {CacheServer: "cache:6379"},
		"unknown parameter":           {CacheServer: "redis://cache:6379?dial_timeout=1s"},
		"bad addr parameter":          {CacheServer: "redis://cache:6379?addr=node-1"},
		"bad database":                {CacheServer: "redis://cache:6379/db"},
		"cluster database":            {CacheServer: "redis://cache:6379/1", CacheCluster: true},
		"cluster and sentinel":        {CacheServer: "redis://cache:6379", CacheCluster: true, CacheSentinelPrimary: "primary"},
		"sentinel without addresses":  {CacheSentinelPrimary: "primary"},
		"cluster without addresses":   {CacheCluster: true},
		"CA file without TLS":         {CacheServer: "redis://cache:6379", CacheCAFile: caFile},
		"unreadable CA file":          {CacheServer: "rediss://cache:6379", CacheCAFile: filepath.Join(t.TempDir(), "missing.pem")},
		"CA file without certificate": {CacheServer: "rediss://cache:6379", CacheCAFile: caFile},
	}
	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := newCacheOptions(&config)
			if err == nil {
				t.Fatal("newCacheOptions returned no error")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("error %q leaks the password", err)
			}
		})
	}
}

func TestNewServer_CacheMode(t *testing.T) {
	tests := map[string]struct {
		config  Config
		cluster bool
	}{
		"standalone":   {config: Config{CacheServer: "redis://cache:6379"}},
		"seed nodes":   {config: Config{CacheServer: "redis://node-0:6379?addr=node-1:6379"}, cluster: true},
		"cluster flag": {config: Config{CacheServer: "redis://cluster:6379", CacheCluster: true}, cluster: true},
		"sentinel":     {config: Config{CacheServer: "redis://sentinel-0:26379?addr=sentinel-1:26379", CacheSentinelPrimary: "primary"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv, err := NewServer(&tt.config, NewMockServer().logger)
			if err != nil {
				t.Fatalf("NewServer returned %v", err)
			}
			defer srv.cache.Close()
			if _, ok := srv.cache.(*redis.ClusterClient); ok != tt.cluster {
				t.Errorf("cluster client is %v, want %v", ok, tt.cluster)
			}
		})
	}
}

func TestNewServer_InvalidCache(t *testing.T) {
	if _, err := NewServer(&Config{CacheServer: "redis://cache:6379", CacheCAFile: "ca.pem"}, NewMockServer().logger); err == nil {
		t.Error("NewServer returned no error")
	}
}

func newCacheMockServer(t *testing.T, config *Config) *Server {
	t.Helper()
	opts, err := newCacheOptions(config)
	if err != nil {
		t.Fatalf("newCacheOptions returned %v", err)
	}
	srv := NewMockServer()
	srv.config.Hostname = config.Hostname
	srv.openCache(opts)
	t.Cleanup(func() { srv.cache.Close() })
	srv.router.HandleFunc("/cache/{key}", srv.cacheWriteHandler).Methods("POST")
	srv.router.HandleFunc("/cache/{key}", srv.cacheDeleteHandler).Methods("DELETE")
	srv.router.HandleFunc("/cache/{key}", srv.cacheReadHandler).Methods("GET")
	return srv
}

func serveCache(srv *Server, method, body string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, "/cache/key1", strings.NewReader(body))
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)
	return rr
}

func TestCacheTracing(t *testing.T) {
	const payload = "cache-payload-4bf92f35"
	for _, cluster := range []bool{false, true} {
		t.Run(map[bool]string{false: "standalone", true: "cluster"}[cluster], func(t *testing.T) {
			spans := setupTracing(t)
			reader := setupMetrics(t)
			m := miniredis.RunT(t)
			opts, err := newCacheOptions(&Config{CacheServer: "redis://" + m.Addr(), CacheCluster: cluster})
			if err != nil {
				t.Fatal(err)
			}
			srv := newInstrumentedServer("/")
			srv.openCache(opts)
			t.Cleanup(func() { srv.cache.Close() })

			rr := httptest.NewRecorder()
			srv.handler.ServeHTTP(rr, httptest.NewRequest("POST", "/cache/key1", strings.NewReader(payload)))
			if rr.Code != http.StatusAccepted {
				t.Fatalf("POST returned %d, want %d", rr.Code, http.StatusAccepted)
			}
			rr = httptest.NewRecorder()
			srv.handler.ServeHTTP(rr, httptest.NewRequest("GET", "/cache/key1", nil))
			if rr.Code != http.StatusOK {
				t.Fatalf("GET returned %d, want %d", rr.Code, http.StatusOK)
			}

			ended := spans.GetSpans()
			servers := map[trace.SpanID]string{}
			for _, s := range spansOfKind(ended, trace.SpanKindServer) {
				servers[s.SpanContext.SpanID()] = s.Name
			}
			commands := map[string]string{}
			for _, s := range ended {
				for _, a := range s.Attributes {
					if strings.Contains(a.Value.Emit(), payload) {
						t.Errorf("span %q attribute %s holds the payload", s.Name, a.Key)
					}
				}
				for _, e := range s.Events {
					for _, a := range e.Attributes {
						if strings.Contains(a.Value.Emit(), payload) {
							t.Errorf("span %q event %q attribute %s holds the payload", s.Name, e.Name, a.Key)
						}
					}
				}
				if s.Name == "set" || s.Name == "get" {
					for _, key := range []attribute.Key{"db.statement", "code.function", "code.filepath", "code.lineno"} {
						if _, ok := attr(s.Attributes, key); ok {
							t.Errorf("span %q has %s", s.Name, key)
						}
					}
					commands[s.Name] = servers[s.Parent.SpanID()]
				}
			}
			want := map[string]string{"set": "POST /cache/{key}", "get": "GET /cache/{key}"}
			if !reflect.DeepEqual(commands, want) {
				t.Errorf("redis spans and their server spans %v, want %v", commands, want)
			}

			var rm metricdata.ResourceMetrics
			if err := reader.Collect(context.Background(), &rm); err != nil {
				t.Fatal(err)
			}
			var found bool
			for _, sm := range rm.ScopeMetrics {
				found = found || sm.Scope.Name == "github.com/redis/go-redis/extra/redisotel" && len(sm.Metrics) > 0
			}
			if !found {
				t.Error("no redisotel metrics recorded")
			}
		})
	}
}
