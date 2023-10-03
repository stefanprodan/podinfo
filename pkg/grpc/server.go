package grpc

import (
	"fmt"
	"net"

	"github.com/stefanprodan/podinfo/pkg/grpc/echo"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/stefanprodan/podinfo/pkg/grpc/version"
	"github.com/stefanprodan/podinfo/pkg/grpc/status"
	"github.com/stefanprodan/podinfo/pkg/grpc/panic"
	"github.com/stefanprodan/podinfo/pkg/grpc/token"
)

type Server struct {
	logger *zap.Logger
	config *Config
}

type Config struct {
	Port        int    `mapstructure:"grpc-port"`
	ServiceName string `mapstructure:"grpc-service-name"`
}

func NewServer(config *Config, logger *zap.Logger) (*Server, error) {
	srv := &Server{
		logger: logger,
		config: config,
	}

	return srv, nil
}


func (s *Server) ListenAndServe() *grpc.Server {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", s.config.Port))
	if err != nil {
		s.logger.Fatal("failed to listen", zap.Int("port", s.config.Port))
	}

	srv := grpc.NewServer()
	server := health.NewServer()
	
	// Register grpc apis
	echo.RegisterEchoServiceServer(srv, &echoServer{})
	version.RegisterVersionServiceServer(srv, &VersionServer{})
	status.RegisterStatusServiceServer(srv, &StatusServer{})
	panic.RegisterPanicServiceServer(srv, &PanicServer{})
	token.RegisterTokenServiceServer(srv, &TokenServer{})

	reflection.Register(srv)
	grpc_health_v1.RegisterHealthServer(srv, server)
	server.SetServingStatus(s.config.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)

	go func() {
		if err := srv.Serve(listener); err != nil {
			s.logger.Fatal("failed to serve", zap.Error(err))
		}
	}()

	return srv
}
