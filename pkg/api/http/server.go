package http

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/stefanprodan/podinfo/pkg/api/http/docs"
	"github.com/stefanprodan/podinfo/pkg/fscache"
	"github.com/stefanprodan/podinfo/pkg/keypair"
	"github.com/stefanprodan/podinfo/pkg/store"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
	httpSwagger "github.com/swaggo/http-swagger"
	"github.com/swaggo/swag"
	"go.uber.org/zap"
)

// @title Podinfo API
// @version 2.0
// @description Go microservice template for Kubernetes.

// @contact.name Source Code
// @contact.url https://github.com/stefanprodan/podinfo

// @license.name Apache License 2.0
// @license.url https://github.com/stefanprodan/podinfo/blob/master/LICENSE

// @BasePath /
// @schemes http https

var (
	healthy int32
	ready   int32
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
	Prefix                string        `mapstructure:"prefix"`
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
	CacheCluster          bool          `mapstructure:"cache-cluster"`
	CacheSentinelPrimary  string        `mapstructure:"cache-sentinel-primary"`
	CacheCAFile           string        `mapstructure:"cache-ca-file"`
	CacheCertPath         string        `mapstructure:"cache-cert-path"`
	CacheUsername         string        `mapstructure:"cache-username"`
	CachePassword         string        `mapstructure:"cache-password"`
	CacheSentinelPassword string        `mapstructure:"cache-sentinel-password"`
	SPIFFEEndpointSocket  string        `mapstructure:"spiffe-endpoint-socket"`
	SPIFFEAuthorizedIDs   []string      `mapstructure:"spiffe-authorized-ids"`
	SPIFFEBackendMTLS     bool          `mapstructure:"spiffe-backend-mtls"`
	SPIFFEBackendIDs      []string      `mapstructure:"spiffe-backend-ids"`
	ArtifactPath          string        `mapstructure:"artifact-path"`
}

type Server struct {
	router       *mux.Router
	logger       *zap.Logger
	config       *Config
	cache        redis.UniversalClient
	cacheClosed  chan struct{}
	cacheStop    chan struct{}
	cacheStopped chan struct{}
	cacheKeyPair *keypair.KeyPair
	handler      http.Handler
	store        *store.Store
	backend      *http.Client
	spiffe       *spiffeState
	artifact     *artifact
	configs      *fscache.Watcher
	keyPair      *keypair.KeyPair
	ui           *uiPage
	metrics      *prometheusMiddleware
}

func NewServer(config *Config, logger *zap.Logger) (*Server, error) {
	config.Prefix = normalizePrefix(config.Prefix)
	docs.SwaggerInfo.BasePath = config.Prefix

	page, err := newUIPage(config)
	if err != nil {
		return nil, fmt.Errorf("loading the UI failed: %w", err)
	}

	spiffe, err := newSPIFFEState(config, logger)
	if err != nil {
		return nil, err
	}

	cacheOpts, cacheKeyPair, err := newCacheOptions(config, logger)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(config.CacheServer, "tcp://") {
		logger.Warn("tcp:// in --cache-server is deprecated, use redis://")
	}

	srv := &Server{
		router:  mux.NewRouter(),
		logger:  logger,
		config:  config,
		spiffe:  spiffe,
		ui:      page,
		store:   store.New(config.DataPath, telemetry.Tracer()),
		backend: newBackendClient(http.DefaultTransport),

		metrics: defaultPrometheusMiddleware(),
	}
	if cacheOpts != nil {
		redis.SetLogger(cacheLogger{logger})
		srv.openCache(cacheOpts)
		srv.cacheKeyPair = cacheKeyPair
	}

	return srv, nil
}

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return "/"
	}

	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}

	prefix = path.Clean(prefix)
	if prefix == "." || prefix == "" {
		return "/"
	}

	return prefix
}

func (s *Server) prefixedPath(route string) string {
	prefix := s.config.Prefix
	if prefix == "" || prefix == "/" {
		return route
	}

	if route == "/" {
		return prefix
	}

	if strings.HasPrefix(route, "/") {
		return prefix + route
	}

	return prefix + "/" + route
}

func (s *Server) registerHandlers() {
	rootPath := s.prefixedPath("/")

	s.router.Handle(s.prefixedPath("/metrics"), s.metrics.metricsHandler())
	s.router.PathPrefix(s.prefixedPath("/debug/pprof/")).Handler(pprofHandler())
	s.router.HandleFunc(rootPath, s.indexHandler).HeadersRegexp("User-Agent", "^Mozilla.*").Methods("GET")
	s.router.HandleFunc(rootPath, s.infoHandler).Methods("GET")
	if rootPath != "/" {
		s.router.HandleFunc(rootPath+"/", s.indexHandler).HeadersRegexp("User-Agent", "^Mozilla.*").Methods("GET")
		s.router.HandleFunc(rootPath+"/", s.infoHandler).Methods("GET")
	}
	s.router.PathPrefix(s.prefixedPath("/ui/")).Handler(http.StripPrefix(s.prefixedPath("/ui"), http.FileServerFS(s.ui.assets))).Methods("GET", "HEAD")
	s.router.HandleFunc(s.prefixedPath("/version"), s.versionHandler).Methods("GET")
	s.router.HandleFunc(s.prefixedPath("/echo"), s.echoHandler)
	s.router.PathPrefix(s.prefixedPath("/echo/")).HandlerFunc(s.echoHandler)
	s.router.HandleFunc(s.prefixedPath("/env"), s.envHandler).Methods("GET", "POST")
	s.router.HandleFunc(s.prefixedPath("/headers"), s.echoHeadersHandler).Methods("GET", "POST")
	s.router.HandleFunc(s.prefixedPath("/delay/{wait:[0-9]+}"), s.delayHandler).Methods("GET").Name("delay")
	s.router.HandleFunc(s.prefixedPath("/healthz"), s.healthzHandler).Methods("GET")
	s.router.HandleFunc(s.prefixedPath("/readyz"), s.readyzHandler).Methods("GET")
	s.router.HandleFunc(s.prefixedPath("/readyz/enable"), s.enableReadyHandler).Methods("POST")
	s.router.HandleFunc(s.prefixedPath("/readyz/disable"), s.disableReadyHandler).Methods("POST")
	s.router.HandleFunc(s.prefixedPath("/fault_injection/enable"), s.enableFaultInjectionHandler).Methods("POST")
	s.router.HandleFunc(s.prefixedPath("/fault_injection/disable"), s.disableFaultInjectionHandler).Methods("POST")
	s.router.HandleFunc(s.prefixedPath("/fault_injection/status"), s.faultInjectionStatusHandler).Methods("GET")
	s.router.HandleFunc(s.prefixedPath("/panic"), s.panicHandler).Methods("GET")
	s.router.HandleFunc(s.prefixedPath("/status/{code:[0-9]+}"), s.statusHandler).Methods("GET", "POST", "PUT").Name("status")
	s.router.HandleFunc(s.prefixedPath("/store"), s.storeWriteHandler).Methods("POST", "PUT")
	s.router.HandleFunc(s.prefixedPath("/store/{hash}"), s.storeReadHandler).Methods("GET").Name("store")
	s.router.HandleFunc(s.prefixedPath("/cache/{key}"), s.cacheWriteHandler).Methods("POST", "PUT")
	s.router.HandleFunc(s.prefixedPath("/cache/{key}"), s.cacheDeleteHandler).Methods("DELETE")
	s.router.HandleFunc(s.prefixedPath("/cache/{key}"), s.cacheReadHandler).Methods("GET").Name("cache")
	s.router.HandleFunc(s.prefixedPath("/configs"), s.configReadHandler).Methods("GET")
	if s.config.ArtifactPath != "" {
		s.router.HandleFunc(s.prefixedPath("/artifact"), s.artifactHandler).Methods("GET", "HEAD")
	}
	s.router.HandleFunc(s.prefixedPath("/token"), s.tokenGenerateHandler).Methods("POST")
	s.router.HandleFunc(s.prefixedPath("/token/validate"), s.tokenValidateHandler).Methods("GET")
	s.router.HandleFunc(s.prefixedPath("/api/info"), s.infoHandler).Methods("GET")
	s.router.HandleFunc(s.prefixedPath("/api/pod"), s.podHandler).Methods("GET")
	s.router.HandleFunc(s.prefixedPath("/api/echo"), s.echoHandler)
	s.router.PathPrefix(s.prefixedPath("/api/echo/")).HandlerFunc(s.echoHandler)
	s.router.HandleFunc(s.prefixedPath("/ws/echo"), s.echoWsHandler)
	s.router.HandleFunc(s.prefixedPath("/chunked"), s.chunkedHandler)
	s.router.HandleFunc(s.prefixedPath("/chunked/{wait:[0-9]+}"), s.chunkedHandler)
	s.router.PathPrefix(s.prefixedPath("/swagger/")).Handler(httpSwagger.Handler(
		httpSwagger.URL(s.prefixedPath("/swagger/doc.json")),
	))
	s.router.HandleFunc(s.prefixedPath("/swagger.json"), func(w http.ResponseWriter, r *http.Request) {
		doc, err := swag.ReadDoc()
		if err != nil {
			s.logger.Error("swagger error", zap.Error(err), zap.String("path", s.prefixedPath("/swagger.json")), telemetry.Context(r.Context()))
		}
		w.Write([]byte(doc))
	})
}

func (s *Server) registerMiddlewares() {
	s.router.Use(routeMiddleware)
	s.router.Use(s.metrics.Handler)
	httpLogger := NewLoggingMiddleware(s.logger)
	s.router.Use(httpLogger.Handler)
	s.router.Use(versionMiddleware)
	s.router.Use(s.faultInjectionMiddleware)
	if s.config.RandomDelay {
		randomDelayer := NewRandomDelayMiddleware(s.config.RandomDelayMin, s.config.RandomDelayMax, s.config.RandomDelayUnit)
		s.router.Use(randomDelayer.Handler)
	}
	if s.config.RandomError {
		s.router.Use(NewRandomErrorMiddleware().Handler)
	}
}

func (s *Server) setupHandler() {
	s.registerHandlers()
	s.registerMiddlewares()
	s.handler = s.instrument(s.router)
}

func (s *Server) ListenAndServe() (*http.Server, *http.Server, *int32, *int32) {
	go s.startMetricsServer()

	s.loadArtifact()
	s.setupHandler()

	// load configs in memory and start watching for changes in the config dir
	if stat, err := os.Stat(s.config.ConfigPath); err == nil && stat.IsDir() {
		configs, err := fscache.New(s.config.ConfigPath, s.logger)
		if err != nil {
			s.logger.Error("config watch error", zap.Error(err), zap.String("path", s.config.ConfigPath))
		} else {
			configs.Start()
			s.configs = configs
		}
	}

	// set the version in the cache on a schedule
	s.startCacheVersion(30 * time.Second)

	// create the http server
	srv := s.startServer()

	// create the secure server
	secureSrv := s.startSecureServer()

	// signal Kubernetes the server is ready to receive traffic
	if !s.config.Unhealthy {
		atomic.StoreInt32(&healthy, 1)
	}
	// with SPIFFE, the secure server sets ready once it has an SVID
	if !s.config.Unready && s.spiffe == nil {
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

	if s.config.H2C {
		srv.Protocols = new(http.Protocols)
		srv.Protocols.SetHTTP1(true)
		srv.Protocols.SetUnencryptedHTTP2(true)
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

	if s.spiffe != nil {
		go s.serveSPIFFE(srv)
		return srv
	}

	if s.keyPair == nil {
		s.logger.Fatal("the HTTPS server has no TLS key pair")
	}
	srv.TLSConfig = s.keyPair.TLSConfig()

	// start the server in the background
	go func() {
		s.logger.Info("Starting HTTPS Server.", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			s.logger.Fatal("HTTPS server crashed", zap.Error(err))
		}
	}()

	// return the server
	return srv
}

func (s *Server) startMetricsServer() {
	if s.config.PortMetrics > 0 {
		if err := s.metricsServer().ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Fatal("metrics server crashed", zap.Error(err))
		}
	}
}

func (s *Server) metricsServer() *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf("%s:%v", s.config.Host, s.config.PortMetrics),
		ReadHeaderTimeout: s.config.HttpServerTimeout,
		Handler:           s.metricsMux(),
	}
}

func (s *Server) metricsMux() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/", pprofHandler())
	mux.Handle("/metrics", s.metrics.metricsHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	return mux
}

// pprofHandler serves the runtime profiles without the cmdline endpoint,
// which would disclose the process arguments, credentials included.
func pprofHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

type ArrayResponse []string
type MapResponse map[string]string

// SetKeyPair makes the HTTPS server serve the --cert-path key pair.
func (s *Server) SetKeyPair(kp *keypair.KeyPair) {
	s.keyPair = kp
}

// Cache returns the --cache-server client, or nil when no cache server is set.
// It stays open until Close.
func (s *Server) Cache() redis.UniversalClient {
	return s.cache
}

// Close releases the cache client and its key pair watcher, the config watcher and the SPIFFE source once the HTTP and gRPC servers have stopped.
func (s *Server) Close() error {
	if s.cacheStop != nil {
		close(s.cacheStop)
		<-s.cacheStopped
	}

	var cacheErr error
	if s.cache != nil {
		cacheErr = s.cache.Close()
		close(s.cacheClosed)
	}

	var cacheKeyPairErr error
	if s.cacheKeyPair != nil {
		cacheKeyPairErr = s.cacheKeyPair.Close()
	}

	var configsErr error
	if s.configs != nil {
		configsErr = s.configs.Close()
	}

	return errors.Join(cacheErr, cacheKeyPairErr, configsErr, s.closeSPIFFE())
}
