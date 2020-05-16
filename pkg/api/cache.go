package api

import (
	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"time"
)

// Cache godoc
// @Summary Save payload in cache
// @Description writes the posted content in cache and returns the SHA1 hash of the content
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /cache [post]
// @Success 200 {object} api.MapResponse
func (s *Server) cacheWriteHandler(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		s.ErrorResponse(w, r, "cache server is offline", http.StatusBadRequest)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.ErrorResponse(w, r, "reading the request body failed", http.StatusBadRequest)
		return
	}

	hash := hash(string(body))

	conn := s.pool.Get()
	defer conn.Close()
	_, err = conn.Do("SET", hash, string(body))
	if err != nil {
		s.logger.Warn("cache set failed", zap.Error(err))
		s.ErrorResponse(w, r, "cache set failed", http.StatusInternalServerError)
		return
	}
	s.JSONResponseCode(w, r, map[string]string{"hash": hash}, http.StatusAccepted)
}

// Cache godoc
// @Summary Get payload from cache
// @Description returns the content from cache if key exists
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /cache/{hash} [get]
// @Success 200 {string} api.MapResponse
func (s *Server) cacheReadHandler(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		s.ErrorResponse(w, r, "cache server is offline", http.StatusBadRequest)
		return
	}

	hash := mux.Vars(r)["hash"]
	conn := s.pool.Get()
	defer conn.Close()

	ok, err := redis.Bool(conn.Do("EXISTS", hash))
	if err != nil || !ok {
		s.ErrorResponse(w, r, "key not found in cache", http.StatusNotFound)
		return
	}

	data, err := redis.String(conn.Do("GET", hash))
	if err != nil {
		s.logger.Warn("cache get failed", zap.Error(err))
		s.ErrorResponse(w, r, "cache get failed", http.StatusInternalServerError)
		return
	}

	s.JSONResponseCode(w, r, map[string]string{"data": data}, http.StatusAccepted)
}

func (s *Server) startCachePool() {
	if s.config.CacheServer != "" {
		s.pool = &redis.Pool{
			MaxIdle:     3,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redis.Conn, error) {
				return redis.Dial("tcp", s.config.CacheServer)
			},
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				_, err := c.Do("PING")
				return err
			},
		}
	}
}
