package http

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

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
func (s *Server) cacheWriteHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "cacheWriteHandler")
	defer span.End()

	if s.pool == nil {
		s.ErrorResponse(w, r, span, "cache server is offline", http.StatusBadRequest)
		return
	}

	key := mux.Vars(r)["key"]
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.ErrorResponse(w, r, span, "reading the request body failed", http.StatusBadRequest)
		return
	}

	conn := s.pool.Get()
	defer conn.Close()
	_, err = conn.Do("SET", key, string(body))
	if err != nil {
		s.logger.Warn("cache set failed", zap.Error(err))
		s.ErrorResponse(w, r, span, "cache set failed", http.StatusInternalServerError)
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
func (s *Server) cacheDeleteHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "cacheDeleteHandler")
	defer span.End()

	if s.pool == nil {
		s.ErrorResponse(w, r, span, "cache server is offline", http.StatusBadRequest)
		return
	}

	key := mux.Vars(r)["key"]

	conn := s.pool.Get()
	defer conn.Close()
	_, err := conn.Do("DEL", key)
	if err != nil {
		s.logger.Warn("cache delete failed", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
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
func (s *Server) cacheReadHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "cacheReadHandler")
	defer span.End()

	if s.pool == nil {
		s.ErrorResponse(w, r, span, "cache server is offline", http.StatusBadRequest)
		return
	}

	key := mux.Vars(r)["key"]

	conn := s.pool.Get()
	defer conn.Close()

	ok, err := redis.Bool(conn.Do("EXISTS", key))
	if err != nil || !ok {
		s.logger.Warn("cache key not found", zap.String("key", key))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	data, err := redis.String(conn.Do("GET", key))
	if err != nil {
		s.logger.Warn("cache get failed", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(data))
}

func (s *Server) getCacheConn() (redis.Conn, error) {
	redisUrl, err := url.Parse(s.config.CacheServer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %v", err)
	}

	var opts []redis.DialOption
	if user := redisUrl.User; user != nil {
		opts = append(opts, redis.DialUsername(user.Username()))
		if password, ok := user.Password(); ok {
			opts = append(opts, redis.DialPassword(password))
		}
	}

	return redis.Dial("tcp", redisUrl.Host, opts...)
}

func (s *Server) startCachePool(ticker *time.Ticker) {
	if s.config.CacheServer == "" {
		return
	}
	s.pool = &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial:        s.getCacheConn,
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}

	// set <hostname>=<version> with an expiry time of one minute
	setVersion := func() {
		conn := s.pool.Get()
		if _, err := conn.Do("SET", s.config.Hostname, version.VERSION, "EX", 60); err != nil {
			s.logger.Warn("cache server is offline", zap.Error(err), zap.String("server", s.config.CacheServer))
		}
		_ = conn.Close()
	}

	// set version on a schedule
	go func() {
		setVersion()
		for {
			select {
			case <-ticker.C:
				setVersion()
			}
		}
	}()
}
