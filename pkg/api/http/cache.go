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

	"github.com/stefanprodan/podinfo/pkg/telemetry"
	"github.com/stefanprodan/podinfo/pkg/version"
)

// Cache godoc
// @Summary Save payload in cache
// @Description writes the posted content in cache
// @Tags HTTP API
// @Accept json
// @Produce json
// @Param key path string true "Key to save to"
// @Router /cache/{key} [post]
// @Success 202
// @Failure 400 {object} http.ErrorMessage "invalid request body"
// @Failure 413 {object} http.ErrorMessage "request body too large"
// @Failure 500 {object} http.ErrorMessage "cache set failed"
// @Failure 503 {object} http.ErrorMessage "cache server is offline"
func (s *Server) cacheWriteHandler(w http.ResponseWriter, r *http.Request) {
	if s.cache == nil {
		s.ErrorResponse(w, r, "cache server is offline", http.StatusServiceUnavailable)
		return
	}

	key := mux.Vars(r)["key"]
	body, ok := s.readLimitedBody(w, r)
	if !ok {
		return
	}

	if err := s.cache.Set(r.Context(), key, body, 0).Err(); err != nil {
		s.logger.Warn("cache set failed", zap.Error(err), telemetry.Context(r.Context()))
		s.ErrorResponse(w, r, "cache set failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Cache godoc
// @Summary Delete payload from cache
// @Description deletes the key and its value from cache
// @Tags HTTP API
// @Accept json
// @Produce json
// @Param key path string true "Key to delete"
// @Router /cache/{key} [delete]
// @Success 202
// @Failure 500 {object} http.ErrorMessage "cache delete failed"
// @Failure 503 {object} http.ErrorMessage "cache server is offline"
func (s *Server) cacheDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if s.cache == nil {
		s.ErrorResponse(w, r, "cache server is offline", http.StatusServiceUnavailable)
		return
	}

	key := mux.Vars(r)["key"]

	if err := s.cache.Del(r.Context(), key).Err(); err != nil {
		s.logger.Warn("cache delete failed", zap.Error(err), telemetry.Context(r.Context()))
		s.ErrorResponse(w, r, "cache delete failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Cache godoc
// @Summary Get payload from cache
// @Description returns the content from cache if key exists
// @Tags HTTP API
// @Accept json
// @Produce json
// @Param key path string true "Key to load from cache"
// @Router /cache/{key} [get]
// @Success 200 {string} string value
// @Failure 404 "key not found"
// @Failure 500 {object} http.ErrorMessage "cache get failed"
// @Failure 503 {object} http.ErrorMessage "cache server is offline"
func (s *Server) cacheReadHandler(w http.ResponseWriter, r *http.Request) {
	if s.cache == nil {
		s.ErrorResponse(w, r, "cache server is offline", http.StatusServiceUnavailable)
		return
	}

	key := mux.Vars(r)["key"]

	data, err := s.cache.Get(r.Context(), key).Bytes()
	if errors.Is(err, redis.Nil) {
		s.logger.Warn("cache key not found", zap.String("key", key), telemetry.Context(r.Context()))
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		s.logger.Warn("cache get failed", zap.Error(err), telemetry.Context(r.Context()))
		s.ErrorResponse(w, r, "cache get failed", http.StatusInternalServerError)
		return
	}

	setRawResponseHeaders(w)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func newCacheOptions(config *Config) (*redis.UniversalOptions, error) {
	if config.CacheServer == "" {
		if config.CacheCluster || config.CacheSentinelPrimary != "" || config.CacheCAFile != "" {
			return nil, errors.New("--cache-cluster, --cache-sentinel-primary and --cache-ca-file require --cache-server")
		}
		return nil, nil
	}
	if config.CacheCluster && config.CacheSentinelPrimary != "" {
		return nil, errors.New("--cache-cluster and --cache-sentinel-primary are mutually exclusive")
	}

	u, err := url.Parse(config.CacheServer)
	if err != nil {
		return nil, fmt.Errorf("invalid cache server URL: %w", errors.Unwrap(err))
	}

	var secure bool
	switch u.Scheme {
	case "redis", "valkey", "tcp":
	case "rediss", "valkeys":
		secure = true
	default:
		return nil, fmt.Errorf("unsupported cache server URL scheme %q, use redis, rediss, valkey or valkeys", u.Scheme)
	}

	if u.Hostname() == "" {
		return nil, errors.New("cache server URL has no host")
	}
	addrs := []string{u.Host}
	if u.Port() == "" {
		addrs[0] = net.JoinHostPort(u.Hostname(), "6379")
	}

	for key, values := range u.Query() {
		if key != "addr" {
			return nil, fmt.Errorf("unsupported cache server URL parameter %q", key)
		}
		for _, addr := range values {
			if host, port, err := net.SplitHostPort(addr); err != nil || host == "" || port == "" {
				return nil, fmt.Errorf("invalid cache server address %q, want host:port", addr)
			}
			addrs = append(addrs, addr)
		}
	}

	var db int
	if path := strings.Trim(u.Path, "/"); path != "" {
		if db, err = strconv.Atoi(path); err != nil || db < 0 {
			return nil, fmt.Errorf("invalid cache server database %q", path)
		}
	}
	if db != 0 && config.CacheSentinelPrimary == "" && (config.CacheCluster || len(addrs) > 1) {
		return nil, errors.New("cluster mode supports only database 0")
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
				return nil, fmt.Errorf("reading the cache CA file failed: %w", err)
			}
			roots := x509.NewCertPool()
			if !roots.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("no PEM certificates found in the cache CA file %s", config.CacheCAFile)
			}
			opts.TLSConfig.RootCAs = roots
		}
	} else if config.CacheCAFile != "" {
		return nil, errors.New("--cache-ca-file requires a rediss:// or valkeys:// cache server URL")
	}

	return opts, nil
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

	// set <hostname>=<version> with an expiry time of one minute
	setVersion := func() {
		if err := s.cache.Set(context.Background(), s.config.Hostname, version.VERSION, time.Minute).Err(); err != nil {
			s.logger.Warn("cache server is offline", zap.Error(err))
		}
	}

	// set version on a schedule
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
