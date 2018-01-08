package server

import (
	"context"
	"io/ioutil"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
)

var status int32

type Server struct {
	mux *http.ServeMux
}

func New(options ...func(*Server)) *Server {
	s := &Server{mux: http.NewServeMux()}

	for _, f := range options {
		f(s)
	}

	s.mux.HandleFunc("/", s.index)
	s.mux.HandleFunc("/healthz/", s.healthz)
	s.mux.HandleFunc("/echo/", s.echo)
	s.mux.HandleFunc("/panic/", s.panic)
	s.mux.Handle("/metrics", promhttp.Handler())

	return s
}

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
	if atomic.LoadInt32(&status) == 1 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) panic(w http.ResponseWriter, r *http.Request) {
	glog.Fatal("Kill switch triggered")
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", runtime.Version())

	s.mux.ServeHTTP(w, r)
}

func ListenAndServe(port string, timeout time.Duration, stopCh <-chan struct{}) {
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      New(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	atomic.StoreInt32(&status, 1)

	// run server in background
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			glog.Fatal(err)
		}
	}()

	// wait for SIGTERM or SIGINT
	<-stopCh
	atomic.StoreInt32(&status, 0)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	glog.Infof("Shutting down HTTP server with timeout: %v", timeout)

	if err := srv.Shutdown(ctx); err != nil {
		glog.Errorf("HTTP server graceful shutdown failed with error: %v", err)
	} else {
		glog.Info("HTTP server stopped")
	}
}
