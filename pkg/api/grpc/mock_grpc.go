package grpc

import (
	"go.uber.org/zap"
)

// NewMockGrpcServer returns a Server for tests, with a fixed config and a
// development logger.
func NewMockGrpcServer() *Server {
	config := &Config{
		Port:        9999,
		ServiceName: "podinfo",
		UIColor:     "blue",
		UIMessage:   "Greetings",
		Hostname:    "localhost",
	}

	logger, _ := zap.NewDevelopment()

	return &Server{
		logger: logger,
		config: config,
	}
}
