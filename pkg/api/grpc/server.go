package grpc

import (
	"fmt"
	"net"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/echo"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/delay"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/env"
	header "github.com/stefanprodan/podinfo/pkg/api/grpc/headers"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/info"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/panic"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/status"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/token"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/version"
)

type Server struct {
	logger *zap.Logger
	config *Config
}

type Config struct {
	Port        int    `mapstructure:"grpc-port"`
	ServiceName string `mapstructure:"grpc-service-name"`

	BackendURL []string `mapstructure:"backend-url"`
	UILogo     string   `mapstructure:"ui-logo"`
	UIMessage  string   `mapstructure:"ui-message"`
	UIColor    string   `mapstructure:"ui-color"`
	UIPath     string   `mapstructure:"ui-path"`
	DataPath   string   `mapstructure:"data-path"`
	ConfigPath string   `mapstructure:"config-path"`
	CertPath   string   `mapstructure:"cert-path"`
	Host       string   `mapstructure:"host"`
	//Port                  string        `mapstructure:"port"`
	SecurePort      string `mapstructure:"secure-port"`
	PortMetrics     int    `mapstructure:"port-metrics"`
	Hostname        string `mapstructure:"hostname"`
	H2C             bool   `mapstructure:"h2c"`
	RandomDelay     bool   `mapstructure:"random-delay"`
	RandomDelayUnit string `mapstructure:"random-delay-unit"`
	RandomDelayMin  int    `mapstructure:"random-delay-min"`
	RandomDelayMax  int    `mapstructure:"random-delay-max"`
	RandomError     bool   `mapstructure:"random-error"`
	Unhealthy       bool   `mapstructure:"unhealthy"`
	Unready         bool   `mapstructure:"unready"`
	JWTSecret       string `mapstructure:"jwt-secret"`
	CacheServer     string `mapstructure:"cache-server"`
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

	// Register grpc apis for reflection
	echo.RegisterEchoServiceServer(srv, &echoServer{config: s.config, logger: s.logger})
	version.RegisterVersionServiceServer(srv, &VersionServer{config: s.config, logger: s.logger})
	panic.RegisterPanicServiceServer(srv, &PanicServer{config: s.config, logger: s.logger})
	delay.RegisterDelayServiceServer(srv, &DelayServer{config: s.config, logger: s.logger})
	header.RegisterHeaderServiceServer(srv, &HeaderServer{config: s.config, logger: s.logger})
	info.RegisterInfoServiceServer(srv, &infoServer{config: s.config})
	status.RegisterStatusServiceServer(srv, &StatusServer{config: s.config, logger: s.logger})
	token.RegisterTokenServiceServer(srv, &TokenServer{config: s.config, logger: s.logger})
	env.RegisterEnvServiceServer(srv, &EnvServer{config: s.config, logger: s.logger})

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
