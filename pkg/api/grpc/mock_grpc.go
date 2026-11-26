package grpc

import (
	"go.uber.org/zap"
)

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
