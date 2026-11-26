package grpc

import (
	"net"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/delay"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/echo"
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
	Host        string `mapstructure:"host"`
	Hostname    string `mapstructure:"hostname"`
	UILogo      string `mapstructure:"ui-logo"`
	UIMessage   string `mapstructure:"ui-message"`
	UIColor     string `mapstructure:"ui-color"`
	Unhealthy   bool   `mapstructure:"unhealthy"`
	Unready     bool   `mapstructure:"unready"`
	JWTSecret   string `mapstructure:"jwt-secret"`
}

func NewServer(config *Config, logger *zap.Logger) *Server {
	return &Server{
		logger: logger,
		config: config,
	}
}

// ListenAndServe starts the gRPC server and returns it with its health server.
func (s *Server) ListenAndServe() (*grpc.Server, *health.Server) {
	address := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		s.logger.Fatal("failed to listen", zap.String("address", address), zap.Error(err))
	}

	srv, healthSrv := s.newServer()

	go func() {
		if err := srv.Serve(listener); err != nil {
			s.logger.Fatal("failed to serve", zap.Error(err))
		}
	}()

	return srv, healthSrv
}

func (s *Server) newServer() (*grpc.Server, *health.Server) {
	srv := grpc.NewServer(s.serverOptions(defaultMetrics())...)

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

	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(srv, healthSrv)

	live, ready := grpc_health_v1.HealthCheckResponse_SERVING, grpc_health_v1.HealthCheckResponse_SERVING
	if s.config.Unhealthy {
		live = grpc_health_v1.HealthCheckResponse_NOT_SERVING
	}
	if s.config.Unhealthy || s.config.Unready {
		ready = grpc_health_v1.HealthCheckResponse_NOT_SERVING
	}
	healthSrv.SetServingStatus("", live)
	if s.config.ServiceName != "" {
		healthSrv.SetServingStatus(s.config.ServiceName, ready)
	}

	return srv, healthSrv
}
