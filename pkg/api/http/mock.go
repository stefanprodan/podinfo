package http

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/store"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

// NewMockServer returns a Server for tests, with a fixed config, a development
// logger and its own Prometheus registry. It has no routes registered and no
// cache.
func NewMockServer() *Server {
	config := &Config{
		Port:                  "9898",
		ServerShutdownTimeout: 5 * time.Second,
		HTTPServerTimeout:     30 * time.Second,
		BackendURL:            []string{},
		ConfigPath:            "/config",
		DataPath:              "/data",
		HTTPClientTimeout:     30 * time.Second,
		UIColor:               "blue",
		UIMessage:             "Greetings",
		Hostname:              "localhost",
		Prefix:                "/",
	}

	page, err := newUIPage(config)
	if err != nil {
		panic(err)
	}

	swagger, err := newSwaggerDocs(config.Prefix)
	if err != nil {
		panic(err)
	}

	logger, _ := zap.NewDevelopment()
	reg := prometheus.NewRegistry()

	return &Server{
		router:  mux.NewRouter(),
		logger:  logger,
		config:  config,
		ui:      page,
		swagger: swagger,
		store:   store.New(config.DataPath, telemetry.Tracer()),
		backend: newBackendClient(http.DefaultTransport),

		metrics: newPrometheusMiddleware(reg, reg),
	}
}
