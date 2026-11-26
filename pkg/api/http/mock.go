package http

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

func NewMockServer() *Server {
	config := &Config{
		Port:                  "9898",
		ServerShutdownTimeout: 5 * time.Second,
		HttpServerTimeout:     30 * time.Second,
		BackendURL:            []string{},
		ConfigPath:            "/config",
		DataPath:              "/data",
		HttpClientTimeout:     30 * time.Second,
		UIColor:               "blue",
		UIMessage:             "Greetings",
		Hostname:              "localhost",
		Prefix:                "/",
	}

	page, err := newUIPage(config)
	if err != nil {
		panic(err)
	}

	logger, _ := zap.NewDevelopment()

	return &Server{
		router:  mux.NewRouter(),
		logger:  logger,
		config:  config,
		ui:      page,
		tracer:  telemetry.Tracer(),
		backend: newBackendClient(http.DefaultTransport),
	}
}
