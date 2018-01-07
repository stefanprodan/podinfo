package server

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
)

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
	}

	w.Header().Set("Content-Type", "text/x-yaml")
	w.Write(d)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", runtime.Version())

	s.mux.ServeHTTP(w, r)
}

func ListenAndServe(port string, timeout time.Duration, stopCh <-chan struct{}) {
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: New(),
	}

	// run server in background
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			glog.Fatal(err)
		}
	}()

	// wait for SIGTERM or SIGINT
	<-stopCh
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	glog.Infof("Shutdown HTTP server with timeout: %s", timeout)

	if err := srv.Shutdown(ctx); err != nil {
		glog.Error(err)
	} else {
		glog.Info("HTTP server stopped")
	}
}
