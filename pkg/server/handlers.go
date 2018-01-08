package server

import (
	"io/ioutil"
	"net/http"
	"sync/atomic"

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
)

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	resp, err := makeResponse()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	d, err := yaml.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "text/x-yaml; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	w.Write(d)
}

func (s *Server) echo(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			glog.Errorf("Reading the request body failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		glog.Infof("Payload received from %s: %s", r.RemoteAddr, string(body))
		w.Write(body)
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotAcceptable)
	}
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&healthy) == 1 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&ready) == 1 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) enable(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt32(&ready, 1)
}

func (s *Server) disable(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt32(&ready, 0)
}

func (s *Server) panic(w http.ResponseWriter, r *http.Request) {
	glog.Fatal("Kill switch triggered")
}
