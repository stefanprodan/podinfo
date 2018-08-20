package api

import (
	"net/http"
	"sync/atomic"
)

func (s *Server) healthzHandler(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&healthy) == 1 {
		s.JSONResponse(w, r, map[string]string{"status": "OK"})
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) readyzHandler(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&ready) == 1 {
		s.JSONResponse(w, r, map[string]string{"status": "OK"})
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) enableReadyHandler(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt32(&ready, 1)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) disableReadyHandler(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt32(&ready, 0)
	w.WriteHeader(http.StatusAccepted)
}
