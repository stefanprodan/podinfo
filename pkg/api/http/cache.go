package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/api/limits"
	"github.com/stefanprodan/podinfo/pkg/keypair"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
	"github.com/stefanprodan/podinfo/pkg/version"
)

// Cache godoc
// @Summary Save payload in cache
// @Description writes the posted content in cache
// @Tags HTTP API
// @Accept json,plain,octet-stream
// @Produce json
// @Param key path string true "Key to save to"
// @Param payload body string false "content to save"
// @Router /cache/{key} [post]
// @Router /cache/{key} [put]
// @Success 202
// @Failure 400 {object} http.ErrorMessage "invalid request body or key too long"
// @Failure 413 {object} http.ErrorMessage "request body too large"
// @Failure 499 {object} http.ErrorMessage "client closed request"
// @Failure 500 {object} http.ErrorMessage "cache set failed"
// @Failure 503 {object} http.ErrorMessage "cache server not configured"
func (s *Server) cacheWriteHandler(w http.ResponseWriter, r *http.Request) {
	key, ok := s.cacheKey(w, r)
	if !ok {
		return
	}
	body, ok := s.readLimitedBody(w, r)
	if !ok {
		return
	}

	if err := s.cache.Set(r.Context(), key, body, 0).Err(); err != nil {
		s.cacheFailed(w, r, "cache set failed", err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Cache godoc
// @Summary Delete payload from cache
// @Description deletes the key and its value from cache
// @Tags HTTP API
// @Produce json
// @Param key path string true "Key to delete"
// @Router /cache/{key} [delete]
// @Success 202
// @Failure 400 {object} http.ErrorMessage "key too long"
// @Failure 499 {object} http.ErrorMessage "client closed request"
// @Failure 500 {object} http.ErrorMessage "cache delete failed"
// @Failure 503 {object} http.ErrorMessage "cache server not configured"
func (s *Server) cacheDeleteHandler(w http.ResponseWriter, r *http.Request) {
	key, ok := s.cacheKey(w, r)
	if !ok {
		return
	}

	if err := s.cache.Del(r.Context(), key).Err(); err != nil {
		s.cacheFailed(w, r, "cache delete failed", err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Cache godoc
// @Summary Get payload from cache
// @Description returns the content from cache if key exists
// @Tags HTTP API
// @Produce octet-stream,json
// @Param key path string true "Key to load from cache"
// @Router /cache/{key} [get]
// @Success 200 {file} file "value"
// @Failure 400 {object} http.ErrorMessage "key too long"
// @Failure 404 "key not found"
// @Failure 499 {object} http.ErrorMessage "client closed request"
// @Failure 500 {object} http.ErrorMessage "cache get failed"
// @Failure 503 {object} http.ErrorMessage "cache server not configured"
func (s *Server) cacheReadHandler(w http.ResponseWriter, r *http.Request) {
	key, ok := s.cacheKey(w, r)
	if !ok {
		return
	}

	data, err := s.cache.Get(r.Context(), key).Bytes()
	if errors.Is(err, redis.Nil) {
		s.logger.Warn("cache key not found", zap.String("key", key), telemetry.Context(r.Context()))
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		s.cacheFailed(w, r, "cache get failed", err)
		return
	}

	setRawResponseHeaders(w)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// cacheKey returns the key of a cache request. It answers 503 without a
// cache and 400 for a key over limits.MaxCacheKeySize.
func (s *Server) cacheKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	if s.cache == nil {
		s.ErrorResponse(w, r, "cache server not configured", http.StatusServiceUnavailable)
		return "", false
	}
	key := mux.Vars(r)["key"]
	if len(key) > limits.MaxCacheKeySize {
		s.ErrorResponse(w, r, "key too long", http.StatusBadRequest)
		return "", false
	}
	return key, true
}

// statusClientClosedRequest is the de facto status for a request the client gave up on.
const statusClientClosedRequest = 499

// cacheFailed answers a failed cache command: 499 when the client canceled
// or timed out, 500 otherwise.
func (s *Server) cacheFailed(w http.ResponseWriter, r *http.Request, msg string, err error) {
	if r.Context().Err() != nil {
		s.ErrorResponse(w, r, "client closed request", statusClientClosedRequest)
		return
	}
	s.logger.Warn(msg, zap.Error(err), telemetry.Context(r.Context()))
	s.ErrorResponse(w, r, msg, http.StatusInternalServerError)
}

func newCacheOptions(config *Config, logger *zap.Logger) (*redis.UniversalOptions, *keypair.KeyPair, error) {
	if config.CacheServer == "" {
		if config.CacheCluster || config.CacheSentinelPrimary != "" || config.CacheCAFile != "" || config.CacheCertPath != "" {
			return nil, nil, errors.New("--cache-cluster, --cache-sentinel-primary, --cache-ca-file and --cache-cert-path require --cache-server")
		}
		return nil, nil, nil
	}
	if config.CacheCluster && config.CacheSentinelPrimary != "" {
		return nil, nil, errors.New("--cache-cluster and --cache-sentinel-primary are mutually exclusive")
	}

	u, err := url.Parse(config.CacheServer)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cache server URL: %w", redactURLError(err, ""))
	}

	var secure bool
	switch u.Scheme {
	case "redis", "valkey", "tcp":
	case "rediss", "valkeys":
		secure = true
	default:
		return nil, nil, fmt.Errorf("unsupported cache server URL scheme %q, use redis, rediss, valkey or valkeys", u.Scheme)
	}

	if u.Hostname() == "" {
		return nil, nil, errors.New("cache server URL has no host")
	}
	addrs := []string{u.Host}
	if u.Port() == "" {
		addrs[0] = net.JoinHostPort(u.Hostname(), "6379")
	}

	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, nil, errors.New("invalid cache server URL query, want addr=host:port pairs separated by '&'")
	}
	for key, values := range query {
		if key != "addr" {
			return nil, nil, fmt.Errorf("unsupported cache server URL parameter %q", key)
		}
		for _, addr := range values {
			if !validCacheAddr(addr) {
				return nil, nil, fmt.Errorf("invalid cache server address %q, want host:port", addr)
			}
			addrs = append(addrs, addr)
		}
	}

	var db int
	if path := strings.Trim(u.Path, "/"); path != "" {
		if db, err = strconv.Atoi(path); err != nil || db < 0 {
			return nil, nil, fmt.Errorf("invalid cache server database %q", path)
		}
	}
	if db != 0 && config.CacheSentinelPrimary == "" && (config.CacheCluster || len(addrs) > 1) {
		return nil, nil, errors.New("cluster mode supports only database 0")
	}

	opts := &redis.UniversalOptions{
		Addrs:            addrs,
		DB:               db,
		IsClusterMode:    config.CacheCluster,
		MasterName:       config.CacheSentinelPrimary,
		Username:         config.CacheUsername,
		Password:         config.CachePassword,
		SentinelPassword: config.CacheSentinelPassword,
	}
	opts.MaintNotificationsConfig = &maintnotifications.Config{Mode: maintnotifications.ModeDisabled}
	if opts.Username == "" {
		opts.Username = u.User.Username()
	}
	if password, ok := u.User.Password(); ok && opts.Password == "" {
		opts.Password = password
	}

	if secure {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		if config.CacheCAFile != "" {
			pem, err := os.ReadFile(config.CacheCAFile)
			if err != nil {
				return nil, nil, fmt.Errorf("reading the cache CA file failed: %w", err)
			}
			roots := x509.NewCertPool()
			if !roots.AppendCertsFromPEM(pem) {
				return nil, nil, fmt.Errorf("no PEM certificates found in the cache CA file %s", config.CacheCAFile)
			}
			opts.TLSConfig.RootCAs = roots
		}
	} else if config.CacheCAFile != "" {
		return nil, nil, errors.New("--cache-ca-file requires a rediss:// or valkeys:// cache server URL")
	} else if config.CacheCertPath != "" {
		return nil, nil, errors.New("--cache-cert-path requires a rediss:// or valkeys:// cache server URL")
	}

	if config.CacheCertPath == "" {
		return opts, nil, nil
	}
	kp, err := keypair.New(config.CacheCertPath, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("loading the cache client key pair from %s failed: %w", config.CacheCertPath, err)
	}
	opts.TLSConfig.GetClientCertificate = kp.GetClientCertificate
	return opts, kp, nil
}

func validCacheAddr(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	n, err := strconv.ParseUint(port, 10, 16)
	return err == nil && n > 0
}

func (s *Server) openCache(opts *redis.UniversalOptions) {
	s.cache = redis.NewUniversalClient(opts)
	s.cacheClosed = make(chan struct{})
	if err := errors.Join(
		redisotel.InstrumentTracing(s.cache, redisotel.WithDBStatement(false), redisotel.WithCallerEnabled(false)),
		redisotel.InstrumentMetrics(s.cache, redisotel.WithCloseChan(s.cacheClosed)),
	); err != nil {
		s.logger.Warn("OpenTelemetry instrumentation of the cache client failed", zap.Error(err))
	}
}

type cacheLogger struct {
	logger *zap.Logger
}

func (l cacheLogger) Printf(ctx context.Context, format string, v ...any) {
	l.logger.Warn(fmt.Sprintf(format, v...), telemetry.Context(ctx))
}

func (s *Server) startCacheVersion(interval time.Duration) {
	if s.cache == nil {
		return
	}

	setVersion := func() {
		if err := s.cache.Set(context.Background(), s.config.Hostname, version.VERSION, time.Minute).Err(); err != nil {
			s.logger.Warn("cache server is offline", zap.Error(err))
		}
	}

	s.cacheStop = make(chan struct{})
	s.cacheStopped = make(chan struct{})
	go func() {
		defer close(s.cacheStopped)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		setVersion()
		for {
			select {
			case <-ticker.C:
				setVersion()
			case <-s.cacheStop:
				return
			}
		}
	}()
}
