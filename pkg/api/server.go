package api

import (
	"context"
	"fmt"
	"github.com/swaggo/swag"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	_ "github.com/stefanprodan/podinfo/pkg/api/docs"
	"github.com/stefanprodan/podinfo/pkg/fscache"
	"github.com/swaggo/http-swagger"
	"go.uber.org/zap"
)

// @title Podinfo API
// @version 2.0
// @description Go microservice template for Kubernetes.

// @contact.name Source Code
// @contact.url https://github.com/stefanprodan/podinfo

// @license.name MIT License
// @license.url https://github.com/stefanprodan/podinfo/blob/master/LICENSE

// @host localhost:9898
// @BasePath /
// @schemes http https

var (
	healthy int32
	ready   int32
	watcher *fscache.Watcher
)

type FluxConfig struct {
	GitUrl    string `mapstructure:"git-url"`
	GitBranch string `mapstructure:"git-branch"`
}

type Config struct {
	HttpClientTimeout         time.Duration `mapstructure:"http-client-timeout"`
	HttpServerTimeout         time.Duration `mapstructure:"http-server-timeout"`
	HttpServerShutdownTimeout time.Duration `mapstructure:"http-server-shutdown-timeout"`
	BackendURL                []string      `mapstructure:"backend-url"`
	UILogo                    string        `mapstructure:"ui-logo"`
	UIMessage                 string        `mapstructure:"ui-message"`
	UIColor                   string        `mapstructure:"ui-color"`
	UIPath                    string        `mapstructure:"ui-path"`
	DataPath                  string        `mapstructure:"data-path"`
	ConfigPath                string        `mapstructure:"config-path"`
	Port                      string        `mapstructure:"port"`
	PortMetrics               int           `mapstructure:"port-metrics"`
	Hostname                  string        `mapstructure:"hostname"`
	H2C                       bool          `mapstructure:"h2c"`
	RandomDelay               bool          `mapstructure:"random-delay"`
	RandomError               bool          `mapstructure:"random-error"`
	Unhealthy                 bool          `mapstructure:"unhealthy"`
	Unready                   bool          `mapstructure:"unready"`
	JWTSecret                 string        `mapstructure:"jwt-secret"`
}

type Server struct {
	router *mux.Router
	logger *zap.Logger
	config *Config
}

func NewServer(config *Config, logger *zap.Logger) (*Server, error) {
	srv := &Server{
		router: mux.NewRouter(),
		logger: logger,
		config: config,
	}

	return srv, nil
}

func (s *Server) registerHandlers() {
	s.router.Handle("/metrics", promhttp.Handler())
	s.router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	s.router.HandleFunc("/", s.indexHandler).HeadersRegexp("User-Agent", "^Mozilla.*").Methods("GET")
	s.router.HandleFunc("/", s.infoHandler).Methods("GET")
	s.router.HandleFunc("/version", s.versionHandler).Methods("GET")
	s.router.HandleFunc("/echo", s.echoHandler).Methods("POST")
	s.router.HandleFunc("/env", s.envHandler).Methods("GET", "POST")
	s.router.HandleFunc("/headers", s.echoHeadersHandler).Methods("GET", "POST")
	s.router.HandleFunc("/delay/{wait:[0-9]+}", s.delayHandler).Methods("GET").Name("delay")
	s.router.HandleFunc("/healthz", s.healthzHandler).Methods("GET")
	s.router.HandleFunc("/readyz", s.readyzHandler).Methods("GET")
	s.router.HandleFunc("/readyz/enable", s.enableReadyHandler).Methods("POST")
	s.router.HandleFunc("/readyz/disable", s.disableReadyHandler).Methods("POST")
	s.router.HandleFunc("/panic", s.panicHandler).Methods("GET")
	s.router.HandleFunc("/status/{code:[0-9]+}", s.statusHandler).Methods("GET", "POST", "PUT").Name("status")
	s.router.HandleFunc("/store", s.storeWriteHandler).Methods("POST")
	s.router.HandleFunc("/store/{hash}", s.storeReadHandler).Methods("GET").Name("store")
	s.router.HandleFunc("/configs", s.configReadHandler).Methods("GET")
	s.router.HandleFunc("/token", s.tokenGenerateHandler).Methods("POST")
	s.router.HandleFunc("/token/validate", s.tokenValidateHandler).Methods("GET")
	s.router.HandleFunc("/api/info", s.infoHandler).Methods("GET")
	s.router.HandleFunc("/api/echo", s.echoHandler).Methods("POST")
	s.router.HandleFunc("/ws/echo", s.echoWsHandler)
	s.router.HandleFunc("/chunked", s.chunkedHandler)
	s.router.HandleFunc("/chunked/{wait:[0-9]+}", s.chunkedHandler)
	s.router.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
	s.router.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
	s.router.HandleFunc("/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		doc, err := swag.ReadDoc()
		if err != nil {
			s.logger.Error("swagger error", zap.Error(err), zap.String("path", "/swagger.json"))
		}
		w.Write([]byte(doc))
	})
}

func (s *Server) registerMiddlewares() {
	prom := NewPrometheusMiddleware()
	s.router.Use(prom.Handler)
	httpLogger := NewLoggingMiddleware(s.logger)
	s.router.Use(httpLogger.Handler)
	s.router.Use(versionMiddleware)
	if s.config.RandomDelay {
		s.router.Use(randomDelayMiddleware)
	}
	if s.config.RandomError {
		s.router.Use(randomErrorMiddleware)
	}
}

func (s *Server) ListenAndServe(stopCh <-chan struct{}) {
	go s.startMetricsServer()

	s.registerHandlers()
	s.registerMiddlewares()

	var handler http.Handler
	if s.config.H2C {
		handler = h2c.NewHandler(s.router, &http2.Server{})
	} else {
		handler = s.router
	}

	srv := &http.Server{
		Addr:         ":" + s.config.Port,
		WriteTimeout: s.config.HttpServerTimeout,
		ReadTimeout:  s.config.HttpServerTimeout,
		IdleTimeout:  2 * s.config.HttpServerTimeout,
		Handler:      handler,
	}

	//s.printRoutes()

	// load configs in memory and start watching for changes in the config dir
	if stat, err := os.Stat(s.config.ConfigPath); err == nil && stat.IsDir() {
		var err error
		watcher, err = fscache.NewWatch(s.config.ConfigPath)
		if err != nil {
			s.logger.Error("config watch error", zap.Error(err), zap.String("path", s.config.ConfigPath))
		} else {
			watcher.Watch()
		}
	}

	// run server in background
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Fatal("HTTP server crashed", zap.Error(err))
		}
	}()

	// signal Kubernetes the server is ready to receive traffic
	if !s.config.Unhealthy {
		atomic.StoreInt32(&healthy, 1)
	}
	if !s.config.Unready {
		atomic.StoreInt32(&ready, 1)
	}

	// wait for SIGTERM or SIGINT
	<-stopCh
	ctx, cancel := context.WithTimeout(context.Background(), s.config.HttpServerShutdownTimeout)
	defer cancel()

	// all calls to /healthz and /readyz will fail from now on
	atomic.StoreInt32(&healthy, 0)
	atomic.StoreInt32(&ready, 0)

	s.logger.Info("Shutting down HTTP server", zap.Duration("timeout", s.config.HttpServerShutdownTimeout))

	// wait for Kubernetes readiness probe to remove this instance from the load balancer
	// the readiness check interval must be lower than the timeout
	if viper.GetString("level") != "debug" {
		time.Sleep(3 * time.Second)
	}

	// attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		s.logger.Warn("HTTP server graceful shutdown failed", zap.Error(err))
	} else {
		s.logger.Info("HTTP server stopped")
	}
}

func (s *Server) startMetricsServer() {
	if s.config.PortMetrics > 0 {
		mux := http.DefaultServeMux
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		srv := &http.Server{
			Addr:    fmt.Sprintf(":%v", s.config.PortMetrics),
			Handler: mux,
		}

		srv.ListenAndServe()
	}
}

func (s *Server) printRoutes() {
	s.router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		pathTemplate, err := route.GetPathTemplate()
		if err == nil {
			fmt.Println("ROUTE:", pathTemplate)
		}
		pathRegexp, err := route.GetPathRegexp()
		if err == nil {
			fmt.Println("Path regexp:", pathRegexp)
		}
		queriesTemplates, err := route.GetQueriesTemplates()
		if err == nil {
			fmt.Println("Queries templates:", strings.Join(queriesTemplates, ","))
		}
		queriesRegexps, err := route.GetQueriesRegexp()
		if err == nil {
			fmt.Println("Queries regexps:", strings.Join(queriesRegexps, ","))
		}
		methods, err := route.GetMethods()
		if err == nil {
			fmt.Println("Methods:", strings.Join(methods, ","))
		}
		fmt.Println()
		return nil
	})
}

type ArrayResponse []string
type MapResponse map[string]string
