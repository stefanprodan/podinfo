package server

import (
	"context"
	"net/http"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	healthy             int32
	ready               int32
	httpRequestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "The total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
)

type Server struct {
	mux *http.ServeMux
}

func NewServer(options ...func(*Server)) *Server {
	s := &Server{mux: http.NewServeMux()}

	for _, f := range options {
		f(s)
	}

	s.mux.HandleFunc("/", s.index)
	s.mux.HandleFunc("/healthz", s.healthz)
	s.mux.HandleFunc("/readyz", s.readyz)
	s.mux.HandleFunc("/readyz/enable", s.enable)
	s.mux.HandleFunc("/readyz/disable", s.disable)
	s.mux.HandleFunc("/echo", s.echo)
	s.mux.HandleFunc("/panic", s.panic)
	s.mux.Handle("/metrics", promhttp.Handler())

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", runtime.Version())

	s.mux.ServeHTTP(w, r)
}

func instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		interceptor := &interceptor{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(interceptor, r)
		var status = strconv.Itoa(interceptor.statusCode)
		httpRequestsCounter.WithLabelValues(r.Method, r.URL.Path, status).Inc()
	})
}

func ListenAndServe(port string, timeout time.Duration, stopCh <-chan struct{}) {
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      instrument(NewServer()),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	atomic.StoreInt32(&healthy, 1)
	atomic.StoreInt32(&ready, 1)

	prometheus.MustRegister(httpRequestsCounter)

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

	// all calls to /healthz and /readyz will fail from now on
	atomic.StoreInt32(&healthy, 0)
	atomic.StoreInt32(&ready, 0)

	glog.Infof("Shutting down HTTP server with timeout: %v", timeout)

	if err := srv.Shutdown(ctx); err != nil {
		glog.Errorf("HTTP server graceful shutdown failed with error: %v", err)
	} else {
		glog.Info("HTTP server stopped")
	}
}
