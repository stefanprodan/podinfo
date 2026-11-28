package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/stefanprodan/podinfo/pkg/api/limits"
	"github.com/stefanprodan/podinfo/pkg/spiffe/spiffetest"
	"github.com/stefanprodan/podinfo/pkg/version"
)

func TestCacheWriteHandler_NoCache(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/cache/{key}", srv.cacheWriteHandler)

	req, _ := http.NewRequest("POST", "/cache/key1", strings.NewReader("value1"))
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server not configured") {
		t.Errorf("expected not configured error, got: %s", rr.Body.String())
	}
}

func TestCacheDeleteHandler_NoCache(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/cache/{key}", srv.cacheDeleteHandler).Methods("DELETE")

	req, _ := http.NewRequest("DELETE", "/cache/key1", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server not configured") {
		t.Errorf("expected not configured error, got: %s", rr.Body.String())
	}
}

func TestCacheReadHandler_NoCache(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/cache/{key}", srv.cacheReadHandler).Methods("GET")

	req, _ := http.NewRequest("GET", "/cache/key1", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server not configured") {
		t.Errorf("expected not configured error, got: %s", rr.Body.String())
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
	cert, caFile := newCacheServerCert(t)
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

func TestCacheMTLS(t *testing.T) {
	cert, caFile := newCacheServerCert(t)
	for _, cluster := range []bool{false, true} {
		t.Run(map[bool]string{false: "standalone", true: "cluster"}[cluster], func(t *testing.T) {
			first, second := spiffetest.NewCA(t), spiffetest.NewCA(t)
			var mu sync.Mutex
			clientCA, peers := first, map[*server.Peer]*spiffetest.CA{}
			m := miniredis.NewMiniRedis()
			err := m.StartTLS(&tls.Config{GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
				mu.Lock()
				defer mu.Unlock()
				pool := x509.NewCertPool()
				for _, c := range clientCA.Bundle(clientID.TrustDomain()).X509Authorities() {
					pool.AddCert(c)
				}
				return &tls.Config{Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}, nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(m.Close)
			m.Server().SetPreHook(func(p *server.Peer, _ string, _ ...string) bool {
				mu.Lock()
				defer mu.Unlock()
				if ca, ok := peers[p]; ok && ca != clientCA {
					p.Close()
					return true
				}
				peers[p] = clientCA
				return false
			})

			dir := t.TempDir()
			writeClientPair(t, dir, first)
			srv := newCacheMockServer(t, &Config{CacheServer: "rediss://" + m.Addr(), CacheCluster: cluster, CacheCAFile: caFile, CacheCertPath: dir})
			if rr := serveCache(srv, "POST", "value1"); rr.Code != http.StatusAccepted {
				t.Fatalf("POST returned %d, want %d", rr.Code, http.StatusAccepted)
			}
			if rr := serveCache(srv, "GET", ""); rr.Code != http.StatusOK || rr.Body.String() != "value1" {
				t.Fatalf("GET returned %d %q, want %d %q", rr.Code, rr.Body.String(), http.StatusOK, "value1")
			}
			if rr := serveCache(srv, "DELETE", ""); rr.Code != http.StatusAccepted {
				t.Fatalf("DELETE returned %d, want %d", rr.Code, http.StatusAccepted)
			}

			anonymous := newCacheMockServer(t, &Config{CacheServer: "rediss://" + m.Addr(), CacheCluster: cluster, CacheCAFile: caFile})
			if rr := serveCache(anonymous, "POST", "value1"); rr.Code != http.StatusInternalServerError {
				t.Errorf("POST without a client certificate returned %d, want %d", rr.Code, http.StatusInternalServerError)
			}
			if err := anonymous.cache.Ping(t.Context()).Err(); err == nil || !strings.Contains(err.Error(), "tls: certificate required") {
				t.Errorf("PING without a client certificate returned %v, want a certificate required alert", err)
			}
			if m.Exists("key1") {
				t.Error("the write without a client certificate reached the cache")
			}

			mu.Lock()
			clientCA = second
			mu.Unlock()
			if rr := serveCache(srv, "POST", "value1"); rr.Code != http.StatusInternalServerError {
				t.Fatalf("POST with a pair from the old CA returned %d, want %d", rr.Code, http.StatusInternalServerError)
			}

			writeClientPair(t, dir, second)
			waitFor(t, "the rotated client key pair", func() bool { return serveCache(srv, "POST", "value2").Code == http.StatusAccepted })
			if got, _ := m.Get("key1"); got != "value2" {
				t.Errorf("cache has %q after the rotation, want %q", got, "value2")
			}
		})
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

func TestCloseReleasesCacheKeyPair(t *testing.T) {
	dir := t.TempDir()
	writeClientPair(t, dir, spiffetest.NewCA(t))
	id := goroutineID()
	watchers := func() int { return startedBy(id, "fscache.(*Watcher).run(") }

	srv, err := NewServer(&Config{CacheServer: "rediss://cache:6379", CacheCertPath: dir}, NewMockServer().logger)
	if err != nil {
		t.Fatalf("NewServer returned %v", err)
	}
	waitFor(t, "the key pair watcher to start", func() bool { return watchers() == 1 })

	if err := srv.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}
	waitFor(t, "the key pair watcher to stop", func() bool { return watchers() == 0 })
}

func TestNewCacheOptions(t *testing.T) {
	_, caFile := newCacheServerCert(t)
	certDir := t.TempDir()
	writeClientPair(t, certDir, spiffetest.NewCA(t))
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
			name:   "IPv6 seed nodes",
			config: Config{CacheServer: "redis://[fd00::1]:6379?addr=[fd00::2]:6379&addr=%5Bfd00::3%5D:6380"},
			want:   redis.UniversalOptions{Addrs: []string{"[fd00::1]:6379", "[fd00::2]:6379", "[fd00::3]:6380"}},
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
		{
			name:   "client key pair",
			config: Config{CacheServer: "rediss://cache:6380", CacheCAFile: caFile, CacheCertPath: certDir},
			want:   redis.UniversalOptions{Addrs: []string{"cache:6380"}},
			tls:    true,
		},
		{
			name:   "cluster client key pair",
			config: Config{CacheServer: "valkeys://node-0:6379?addr=node-1:6379", CacheCertPath: certDir},
			want:   redis.UniversalOptions{Addrs: []string{"node-0:6379", "node-1:6379"}},
			tls:    true,
		},
		{
			name: "sentinel client key pair",
			config: Config{
				CacheServer:          "rediss://sentinel-0:26379?addr=sentinel-1:26379",
				CacheSentinelPrimary: "primary",
				CacheCAFile:          caFile,
				CacheCertPath:        certDir,
			},
			want: redis.UniversalOptions{Addrs: []string{"sentinel-0:26379", "sentinel-1:26379"}, MasterName: "primary"},
			tls:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, kp, err := newCacheOptions(&tt.config, zap.NewNop())
			if err != nil {
				t.Fatalf("newCacheOptions returned %v", err)
			}
			if (got.TLSConfig != nil) != tt.tls {
				t.Errorf("TLS is %v, want %v", got.TLSConfig != nil, tt.tls)
			}
			if tt.config.CacheCAFile != "" && got.TLSConfig.RootCAs == nil {
				t.Error("the CA file is not applied")
			}
			if (kp != nil) != (tt.config.CacheCertPath != "") {
				t.Fatalf("key pair is %v, want %v", kp != nil, tt.config.CacheCertPath != "")
			}
			if kp != nil {
				t.Cleanup(func() { _ = kp.Close() })
				if got.TLSConfig.GetClientCertificate == nil {
					t.Fatal("GetClientCertificate is not set")
				}
				if cert, _ := got.TLSConfig.GetClientCertificate(nil); cert == nil {
					t.Error("GetClientCertificate returned no certificate")
				}
			} else if got.TLSConfig != nil && got.TLSConfig.GetClientCertificate != nil {
				t.Error("GetClientCertificate is set without a key pair")
			}
			if got.MaintNotificationsConfig == nil || got.MaintNotificationsConfig.Mode != maintnotifications.ModeDisabled {
				t.Errorf("maintenance notifications config %+v, want mode %q", got.MaintNotificationsConfig, maintnotifications.ModeDisabled)
			}
			got.TLSConfig, got.MaintNotificationsConfig = nil, nil
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("newCacheOptions returned %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestNewCacheOptions_Disabled(t *testing.T) {
	opts, kp, err := newCacheOptions(&Config{}, zap.NewNop())
	if opts != nil || kp != nil || err != nil {
		t.Errorf("newCacheOptions returned %v, %v, %v, want nil, nil, nil", opts, kp, err)
	}
}

func TestNewCacheOptions_Invalid(t *testing.T) {
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	certDir := t.TempDir()
	writeClientPair(t, certDir, spiffetest.NewCA(t))
	invalidDir := t.TempDir()
	for _, name := range []string{"tls.crt", "tls.key"} {
		if err := os.WriteFile(filepath.Join(invalidDir, name), []byte("not a certificate"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tests := map[string]Config{
		"bad URL":                     {CacheServer: "redis://user:secret@cache:port"},
		"unknown scheme":              {CacheServer: "memcached://cache:11211"},
		"no host":                     {CacheServer: "redis://:6379"},
		"bare address":                {CacheServer: "cache:6379"},
		"unknown parameter":           {CacheServer: "redis://cache:6379?dial_timeout=1s"},
		"bad addr parameter":          {CacheServer: "redis://cache:6379?addr=node-1"},
		"addr without host":           {CacheServer: "redis://cache:6379?addr=:6379"},
		"addr port not numeric":       {CacheServer: "redis://cache:6379?addr=node-1:abc"},
		"addr port zero":              {CacheServer: "redis://cache:6379?addr=node-1:0"},
		"addr port out of range":      {CacheServer: "redis://cache:6379?addr=node-1:65536"},
		"addr IPv6 without brackets":  {CacheServer: "redis://cache:6379?addr=fd00::2:6379"},
		"semicolon separator":         {CacheServer: "redis://node-0:6379?addr=node-1:6379;addr=node-2:6379"},
		"bad query escape":            {CacheServer: "redis://cache:6379?addr=node-1:6379&x=secret%ZZ"},
		"bad database":                {CacheServer: "redis://cache:6379/db"},
		"cluster database":            {CacheServer: "redis://cache:6379/1", CacheCluster: true},
		"cluster and sentinel":        {CacheServer: "redis://cache:6379", CacheCluster: true, CacheSentinelPrimary: "primary"},
		"sentinel without addresses":  {CacheSentinelPrimary: "primary"},
		"cluster without addresses":   {CacheCluster: true},
		"CA file without TLS":         {CacheServer: "redis://cache:6379", CacheCAFile: caFile},
		"unreadable CA file":          {CacheServer: "rediss://cache:6379", CacheCAFile: filepath.Join(t.TempDir(), "missing.pem")},
		"CA file without certificate": {CacheServer: "rediss://cache:6379", CacheCAFile: caFile},
		"cert path without TLS":       {CacheServer: "redis://cache:6379", CacheCertPath: certDir},
		"cert path without addresses": {CacheCertPath: certDir},
		"missing cert path":           {CacheServer: "rediss://cache:6379", CacheCertPath: filepath.Join(t.TempDir(), "missing")},
		"invalid key pair":            {CacheServer: "rediss://cache:6379", CacheCertPath: invalidDir},
	}
	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, err := newCacheOptions(&config, zap.NewNop())
			if err == nil {
				t.Fatal("newCacheOptions returned no error")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("error %q leaks the password", err)
			}
		})
	}
}

func TestNewCacheOptions_InvalidURLHidesPassword(t *testing.T) {
	tests := []string{
		"redis://user:s3cr3t/abc@host:6379",
		"redis://user:s3%ZZcr3t@host",
		"redis://user:s3cr3t@host:port",
		"redis://host:6379?addr=node-1:6379&password=s3%ZZcr3t",
		"redis://host:6379?addr=node-1:6379;password=s3cr3t",
	}
	for _, server := range tests {
		t.Run(server, func(t *testing.T) {
			_, _, err := newCacheOptions(&Config{CacheServer: server}, zap.NewNop())
			if err == nil {
				t.Fatal("newCacheOptions returned no error")
			}
			for _, part := range []string{"s3", "cr3t", "ZZ", "abc"} {
				if strings.Contains(err.Error(), part) {
					t.Errorf("error %q quotes %q from the URL", err, part)
				}
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
			defer srv.Close()
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
	if _, err := NewServer(&Config{CacheServer: "rediss://cache:6379", CacheCertPath: t.TempDir()}, NewMockServer().logger); err == nil {
		t.Error("NewServer returned no error without a client key pair")
	}
}

func TestServerCache(t *testing.T) {
	if c := NewMockServer().Cache(); c != nil {
		t.Errorf("Cache returned %v without --cache-server, want nil", c)
	}

	m := miniredis.RunT(t)
	srv, err := NewServer(&Config{CacheServer: "redis://" + m.Addr()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewServer returned %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	if err := m.Set("key1", "value1"); err != nil {
		t.Fatal(err)
	}
	if got, err := srv.Cache().Get(t.Context(), "key1").Result(); err != nil || got != "value1" {
		t.Errorf("Get returned %q, %v, want %q", got, err, "value1")
	}
}

func TestCacheHandlers_KeyTooLong(t *testing.T) {
	m := miniredis.RunT(t)
	srv := newCacheMockServer(t, &Config{CacheServer: "redis://" + m.Addr()})

	key := strings.Repeat("k", limits.MaxCacheKeySize)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, httptest.NewRequest("POST", "/cache/"+key, strings.NewReader("value1")))
	if rr.Code != http.StatusAccepted || !m.Exists(key) {
		t.Errorf("POST with a %d byte key returned %d", len(key), rr.Code)
	}

	for _, method := range []string{"POST", "GET", "DELETE"} {
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, httptest.NewRequest(method, "/cache/"+key+"k", strings.NewReader("value1")))
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "key too long") {
			t.Errorf("%s returned %d %s, want 400 key too long", method, rr.Code, rr.Body.String())
		}
	}
}

func TestCacheHandlers_ClientGone(t *testing.T) {
	m := miniredis.RunT(t)
	logs, observed := observer.New(zapcore.WarnLevel)
	srv := newCacheMockServer(t, &Config{CacheServer: "redis://" + m.Addr()})
	srv.logger = zap.New(logs)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, method := range []string{"POST", "GET", "DELETE"} {
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, httptest.NewRequest(method, "/cache/key1", strings.NewReader("value1")).WithContext(ctx))
		if rr.Code != statusClientClosedRequest {
			t.Errorf("%s returned %d, want %d", method, rr.Code, statusClientClosedRequest)
		}
	}
	if n := observed.Len(); n != 0 {
		t.Errorf("got %d warnings for canceled requests: %v", n, observed.All())
	}
}

func newCacheMockServer(t *testing.T, config *Config) *Server {
	t.Helper()
	opts, kp, err := newCacheOptions(config, zap.NewNop())
	if err != nil {
		t.Fatalf("newCacheOptions returned %v", err)
	}
	if kp != nil {
		t.Cleanup(func() { _ = kp.Close() })
	}
	srv := NewMockServer()
	srv.config.Hostname = config.Hostname
	srv.openCache(opts)
	t.Cleanup(func() {
		srv.cache.Close()
		select {
		case <-srv.cacheClosed:
		default:
			close(srv.cacheClosed)
		}
	})
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

func newCacheServerCert(t *testing.T) (tls.Certificate, string) {
	t.Helper()
	ts := httptest.NewTLSServer(nil)
	defer ts.Close()
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	return ts.TLS.Certificates[0], caFile
}

func writeClientPair(t *testing.T, dir string, ca *spiffetest.CA) {
	t.Helper()
	cert, key, err := ca.NewSVID(t, clientID).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"tls.crt": cert, "tls.key": key} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCacheTracing(t *testing.T) {
	const payload = "cache-payload-4bf92f35"
	for _, cluster := range []bool{false, true} {
		t.Run(map[bool]string{false: "standalone", true: "cluster"}[cluster], func(t *testing.T) {
			spans := setupTracing(t)
			reader := setupMetrics(t)
			m := miniredis.RunT(t)
			opts, _, err := newCacheOptions(&Config{CacheServer: "redis://" + m.Addr(), CacheCluster: cluster}, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			srv := newInstrumentedServer("/")
			srv.openCache(opts)
			t.Cleanup(func() { _ = srv.Close() })

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

func TestCacheHandshake(t *testing.T) {
	for _, cluster := range []bool{false, true} {
		t.Run(map[bool]string{false: "standalone", true: "cluster"}[cluster], func(t *testing.T) {
			spans := setupTracing(t)
			m := miniredis.RunT(t)
			var mu sync.Mutex
			var commands []string
			m.Server().SetPreHook(func(_ *server.Peer, cmd string, args ...string) bool {
				mu.Lock()
				defer mu.Unlock()
				commands = append(commands, strings.ToUpper(strings.Join(append([]string{cmd}, args...), " ")))
				return false
			})
			srv := newCacheMockServer(t, &Config{CacheServer: "redis://" + m.Addr(), CacheCluster: cluster})

			if rr := serveCache(srv, "POST", "value1"); rr.Code != http.StatusAccepted {
				t.Fatalf("POST returned %d, want %d", rr.Code, http.StatusAccepted)
			}

			mu.Lock()
			defer mu.Unlock()
			if !slices.Contains(commands, "HELLO 3") {
				t.Errorf("no HELLO 3 in %q", commands)
			}
			for _, c := range commands {
				if strings.HasPrefix(c, "CLIENT MAINT_NOTIFICATIONS") {
					t.Errorf("handshake sent %q", c)
				}
			}
			for _, s := range spans.GetSpans() {
				if strings.Contains(strings.ToUpper(s.Status.Description), "MAINT_NOTIFICATIONS") {
					t.Errorf("span %q status %q", s.Name, s.Status.Description)
				}
			}
		})
	}
}
