package server

import (
	"context"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"github.com/stefanprodan/k8s-podinfo/pkg/fscache"
)

var (
	healthy  int32
	ready    int32
	dataPath string
	watcher  *fscache.Watcher
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
	s.mux.HandleFunc("/echoheaders", s.echoHeaders)
	s.mux.HandleFunc("/backend", s.backend)
	s.mux.HandleFunc("/job", s.job)
	s.mux.HandleFunc("/read", s.read)
	s.mux.HandleFunc("/write", s.write)
	s.mux.HandleFunc("/error", s.error)
	s.mux.HandleFunc("/panic", s.panic)
	s.mux.HandleFunc("/configs", s.configs)
	s.mux.HandleFunc("/version", s.version)
	s.mux.Handle("/metrics", promhttp.Handler())

	// Register pprof handlers
	s.mux.HandleFunc("/debug/pprof/", pprof.Index)
	s.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// API
	s.mux.HandleFunc("/api/info", s.apiInfo)
	s.mux.HandleFunc("/api/echo", s.apiEcho)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", runtime.Version())

	s.mux.ServeHTTP(w, r)
}

func ListenAndServe(port string, timeout time.Duration, stopCh <-chan struct{}) {
	inst := NewInstrument()
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      inst.Wrap(NewServer()),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 1 * time.Minute,
		IdleTimeout:  15 * time.Second,
	}

	atomic.StoreInt32(&healthy, 1)
	atomic.StoreInt32(&ready, 1)

	// local storage path
	dataPath = os.Getenv("data")
	if len(dataPath) < 1 {
		dataPath = "/data"
	}

	// config path
	configPath := os.Getenv("configPath")
	if len(configPath) > 0 {
		var err error
		watcher, err = fscache.NewWatch(configPath)
		if err != nil {
			log.Error().Err(err).Msgf("%s watch error", configPath)
		} else {
			watcher.Watch()
		}
	}

	// run server in background
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server crashed")
		}
	}()

	// wait for SIGTERM or SIGINT
	<-stopCh
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// all calls to /healthz and /readyz will fail from now on
	atomic.StoreInt32(&healthy, 0)
	atomic.StoreInt32(&ready, 0)

	log.Info().Msgf("Shutting down HTTP server with timeout: %v", timeout)

	// wait for Kubernetes readiness probe
	// to remove this instance from the load balancer
	// the readiness check interval must lower than the timeout
	time.Sleep(timeout)

	// attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("HTTP server graceful shutdown failed")
	} else {
		log.Info().Msg("HTTP server stopped")
	}
}
