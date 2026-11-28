package http

import (
	"net/http"
	"sync/atomic"
)

// Healthz godoc
// @Summary Liveness check
// @Description used by Kubernetes liveness probe
// @Tags Kubernetes
// @Produce json
// @Router /healthz [get]
// @Success 200 {object} map[string]string
// @Failure 503 "unhealthy"
func (s *Server) healthzHandler(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&healthy) == 1 {
		s.JSONResponse(w, r, map[string]string{"status": "OK"})
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

// Readyz godoc
// @Summary Readiness check
// @Description used by Kubernetes readiness probe
// @Tags Kubernetes
// @Produce json
// @Router /readyz [get]
// @Success 200 {object} map[string]string
// @Failure 503 "not ready"
func (s *Server) readyzHandler(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&ready) == 1 {
		s.JSONResponse(w, r, map[string]string{"status": "OK"})
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

// EnableReady godoc
// @Summary Enable ready state
// @Description makes the readiness check pass, so Kubernetes sends traffic to this instance
// @Tags Kubernetes
// @Router /readyz/enable [post]
// @Success 202
func (s *Server) enableReadyHandler(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt32(&ready, 1)
	w.WriteHeader(http.StatusAccepted)
}

// DisableReady godoc
// @Summary Disable ready state
// @Description makes the readiness check fail, so Kubernetes stops sending traffic to this instance
// @Tags Kubernetes
// @Router /readyz/disable [post]
// @Success 202
func (s *Server) disableReadyHandler(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt32(&ready, 0)
	w.WriteHeader(http.StatusAccepted)
}
