package http

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "github.com/stefanprodan/podinfo/pkg/api/http/docs"
	"github.com/stefanprodan/podinfo/pkg/fscache"
	httpSwagger "github.com/swaggo/http-swagger"
	"github.com/swaggo/swag"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// @title Podinfo API
// @version 2.0
// @description Go microservice template for Kubernetes.

// @contact.name Source Code
// @contact.url https://github.com/stefanprodan/podinfo

// @license.name MIT License
// @license.url https://github.com/stefanprodan/podinfo/blob/master/LICENSE

// @BasePath /
// @schemes http https

var (
	healthy int32
	ready   int32
	watcher *fscache.Watcher
)

type Config struct {
	HttpClientTimeout     time.Duration `mapstructure:"http-client-timeout"`
	HttpServerTimeout     time.Duration `mapstructure:"http-server-timeout"`
	ServerShutdownTimeout time.Duration `mapstructure:"server-shutdown-timeout"`
	BackendURL            []string      `mapstructure:"backend-url"`
	UILogo                string        `mapstructure:"ui-logo"`
	UIMessage             string        `mapstructure:"ui-message"`
	UIColor               string        `mapstructure:"ui-color"`
	UIPath                string        `mapstructure:"ui-path"`
	DataPath              string        `mapstructure:"data-path"`
	ConfigPath            string        `mapstructure:"config-path"`
	CertPath              string        `mapstructure:"cert-path"`
	Host                  string        `mapstructure:"host"`
	Port                  string        `mapstructure:"port"`
	SecurePort            string        `mapstructure:"secure-port"`
	PortMetrics           int           `mapstructure:"port-metrics"`
	Hostname              string        `mapstructure:"hostname"`
	H2C                   bool          `mapstructure:"h2c"`
	RandomDelay           bool          `mapstructure:"random-delay"`
	RandomDelayUnit       string        `mapstructure:"random-delay-unit"`
	RandomDelayMin        int           `mapstructure:"random-delay-min"`
	RandomDelayMax        int           `mapstructure:"random-delay-max"`
	RandomError           bool          `mapstructure:"random-error"`
	Unhealthy             bool          `mapstructure:"unhealthy"`
	Unready               bool          `mapstructure:"unready"`
	JWTSecret             string        `mapstructure:"jwt-secret"`
	CacheServer           string        `mapstructure:"cache-server"`
}

type Server struct {
	router         *mux.Router
	logger         *zap.Logger
	config         *Config
	pool           *redis.Pool
	handler        http.Handler
	tracer         trace.Tracer
	tracerProvider *sdktrace.TracerProvider
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
	s.router.HandleFunc("/echo", s.echoHandler)
	s.router.PathPrefix("/echo/").HandlerFunc(s.echoHandler)
	s.router.HandleFunc("/env", s.envHandler).Methods("GET", "POST")
	s.router.HandleFunc("/headers", s.echoHeadersHandler).Methods("GET", "POST")
	s.router.HandleFunc("/delay/{wait:[0-9]+}", s.delayHandler).Methods("GET").Name("delay")
	s.router.HandleFunc("/healthz", s.healthzHandler).Methods("GET")
	s.router.HandleFunc("/readyz", s.readyzHandler).Methods("GET")
	s.router.HandleFunc("/readyz/enable", s.enableReadyHandler).Methods("POST")
	s.router.HandleFunc("/readyz/disable", s.disableReadyHandler).Methods("POST")
	s.router.HandleFunc("/panic", s.panicHandler).Methods("GET")
	s.router.HandleFunc("/status/{code:[0-9]+}", s.statusHandler).Methods("GET", "POST", "PUT").Name("status")
	s.router.HandleFunc("/store", s.storeWriteHandler).Methods("POST", "PUT")
	s.router.HandleFunc("/store/{hash}", s.storeReadHandler).Methods("GET").Name("store")
	s.router.HandleFunc("/cache/{key}", s.cacheWriteHandler).Methods("POST", "PUT")
	s.router.HandleFunc("/cache/{key}", s.cacheDeleteHandler).Methods("DELETE")
	s.router.HandleFunc("/cache/{key}", s.cacheReadHandler).Methods("GET").Name("cache")
	s.router.HandleFunc("/configs", s.configReadHandler).Methods("GET")
	s.router.HandleFunc("/token", s.tokenGenerateHandler).Methods("POST")
	s.router.HandleFunc("/token/validate", s.tokenValidateHandler).Methods("GET")
	s.router.HandleFunc("/api/info", s.infoHandler).Methods("GET")
	s.router.HandleFunc("/api/echo", s.echoHandler)
	s.router.PathPrefix("/api/echo/").HandlerFunc(s.echoHandler)
	s.router.HandleFunc("/ws/echo", s.echoWsHandler)
	s.router.HandleFunc("/chunked", s.chunkedHandler)
	s.router.HandleFunc("/chunked/{wait:[0-9]+}", s.chunkedHandler)
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
	otel := NewOpenTelemetryMiddleware()
	s.router.Use(otel)
	httpLogger := NewLoggingMiddleware(s.logger)
	s.router.Use(httpLogger.Handler)
	s.router.Use(versionMiddleware)
	if s.config.RandomDelay {
		randomDelayer := NewRandomDelayMiddleware(s.config.RandomDelayMin, s.config.RandomDelayMax, s.config.RandomDelayUnit)
		s.router.Use(randomDelayer.Handler)
	}
	if s.config.RandomError {
		s.router.Use(randomErrorMiddleware)
	}
}

func (s *Server) ListenAndServe() (*http.Server, *http.Server, *int32, *int32) {
	ctx := context.Background()

	go s.startMetricsServer()

	s.initTracer(ctx)
	s.registerHandlers()
	s.registerMiddlewares()

	if s.config.H2C {
		s.handler = h2c.NewHandler(s.router, &http2.Server{})
	} else {
		s.handler = s.router
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

	// start redis connection pool
	ticker := time.NewTicker(30 * time.Second)
	s.startCachePool(ticker)

	// create the http server
	srv := s.startServer()

	// create the secure server
	secureSrv := s.startSecureServer()

	// signal Kubernetes the server is ready to receive traffic
	if !s.config.Unhealthy {
		atomic.StoreInt32(&healthy, 1)
	}
	if !s.config.Unready {
		atomic.StoreInt32(&ready, 1)
	}

	return srv, secureSrv, &healthy, &ready
}

func (s *Server) startServer() *http.Server {

	// determine if the port is specified
	if s.config.Port == "0" {

		// move on immediately
		return nil
	}

	srv := &http.Server{
		Addr:         s.config.Host + ":" + s.config.Port,
		WriteTimeout: s.config.HttpServerTimeout,
		ReadTimeout:  s.config.HttpServerTimeout,
		IdleTimeout:  2 * s.config.HttpServerTimeout,
		Handler:      s.handler,
	}

	// start the server in the background
	go func() {
		s.logger.Info("Starting HTTP Server.", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Fatal("HTTP server crashed", zap.Error(err))
		}
	}()

	// return the server and routine
	return srv
}

func (s *Server) startSecureServer() *http.Server {

	// determine if the port is specified
	if s.config.SecurePort == "0" {

		// move on immediately
		return nil
	}

	srv := &http.Server{
		Addr:         s.config.Host + ":" + s.config.SecurePort,
		WriteTimeout: s.config.HttpServerTimeout,
		ReadTimeout:  s.config.HttpServerTimeout,
		IdleTimeout:  2 * s.config.HttpServerTimeout,
		Handler:      s.handler,
	}

	cert := path.Join(s.config.CertPath, "tls.crt")
	key := path.Join(s.config.CertPath, "tls.key")

	// start the server in the background
	go func() {
		s.logger.Info("Starting HTTPS Server.", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServeTLS(cert, key); err != http.ErrServerClosed {
			s.logger.Fatal("HTTPS server crashed", zap.Error(err))
		}
	}()

	// return the server
	return srv
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
