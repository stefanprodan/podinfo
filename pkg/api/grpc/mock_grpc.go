package grpc

import (
	"go.uber.org/zap"
)

func NewMockGrpcServer() *Server {
	config := &Config{
		Port: 9999,
		// ServerShutdownTimeout: 5 * time.Second,
		// HttpServerTimeout:     30 * time.Second,
		BackendURL: []string{},
		ConfigPath: "/config",
		DataPath:   "/data",
		// HttpClientTimeout:     30 * time.Second,
		UIColor:   "blue",
		UIPath:    ".ui",
		UIMessage: "Greetings",
		Hostname:  "localhost",
	}

	logger, _ := zap.NewDevelopment()

	return &Server{
		//router: mux.NewRouter(),
		logger: logger,
		config: config,
		//tracer: trace.NewNoopTracerProvider().Tracer("mock"),
	}
}
