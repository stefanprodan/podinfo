package grpc

import (
	"errors"
	"net"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/delay"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/echo"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/env"
	header "github.com/stefanprodan/podinfo/pkg/api/grpc/headers"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/info"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/panic"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/pod"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/status"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/token"
	"github.com/stefanprodan/podinfo/pkg/api/grpc/version"
	"github.com/stefanprodan/podinfo/pkg/keypair"
	"github.com/stefanprodan/podinfo/pkg/spiffe"
)

type Server struct {
	logger  *zap.Logger
	config  *Config
	spiffe  *spiffe.Source
	keyPair *keypair.KeyPair
}

type Config struct {
	Port        int    `mapstructure:"grpc-port"`
	TLS         bool   `mapstructure:"grpc-tls"`
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

// SetSPIFFESource makes --grpc-tls serve SPIFFE mTLS with the source's SVID
// instead of the --cert-path certificate.
func (s *Server) SetSPIFFESource(source *spiffe.Source) {
	s.spiffe = source
}

// SetKeyPair makes --grpc-tls serve the --cert-path key pair.
func (s *Server) SetKeyPair(kp *keypair.KeyPair) {
	s.keyPair = kp
}

// ListenAndServe starts the gRPC server and returns it with its health server.
func (s *Server) ListenAndServe() (*grpc.Server, *health.Server) {
	creds, err := s.credentials()
	if err != nil {
		s.logger.Fatal("failed to load the gRPC TLS certificate", zap.Error(err))
	}

	address := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		s.logger.Fatal("failed to listen", zap.String("address", address), zap.Error(err))
	}

	srv, healthSrv := s.newServer(grpc.Creds(creds))
	if s.spiffeTLS() {
		s.waitForSVID(healthSrv)
	}

	s.logger.Info("Starting gRPC Server",
		zap.String("addr", address),
		zap.Bool("tls", s.config.TLS),
		zap.Bool("spiffe", s.spiffeTLS()))

	go func() {
		if err := srv.Serve(listener); err != nil {
			s.logger.Fatal("failed to serve", zap.Error(err))
		}
	}()

	return srv, healthSrv
}

func (s *Server) newServer(opts ...grpc.ServerOption) (*grpc.Server, *health.Server) {
	srv := grpc.NewServer(append(s.serverOptions(defaultMetrics()), opts...)...)

	echo.RegisterEchoServiceServer(srv, &echoServer{config: s.config, logger: s.logger})
	version.RegisterVersionServiceServer(srv, &VersionServer{config: s.config, logger: s.logger})
	panic.RegisterPanicServiceServer(srv, &PanicServer{config: s.config, logger: s.logger})
	delay.RegisterDelayServiceServer(srv, &DelayServer{config: s.config, logger: s.logger})
	header.RegisterHeaderServiceServer(srv, &HeaderServer{config: s.config, logger: s.logger})
	info.RegisterInfoServiceServer(srv, &infoServer{config: s.config})
	pod.RegisterPodServiceServer(srv, &podServer{config: s.config})
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

func (s *Server) spiffeTLS() bool {
	return s.config.TLS && s.spiffe != nil
}

// credentials returns the transport credentials for --grpc-tls: SPIFFE mTLS
// with the shared source, or server-side TLS with the --cert-path key pair.
func (s *Server) credentials() (credentials.TransportCredentials, error) {
	switch {
	case !s.config.TLS:
		return insecure.NewCredentials(), nil
	case s.spiffe != nil:
		return credentials.NewTLS(s.spiffe.ServerTLSConfig()), nil
	case s.keyPair != nil:
		return credentials.NewTLS(s.keyPair.TLSConfig()), nil
	}
	return nil, errors.New("--grpc-tls requires a TLS key pair")
}

// waitForSVID reports the service NOT_SERVING until the SPIFFE SVID is issued,
// since TLS handshakes fail until then.
func (s *Server) waitForSVID(healthSrv *health.Server) {
	if s.config.ServiceName != "" {
		healthSrv.SetServingStatus(s.config.ServiceName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	}
	go func() {
		id, err := s.spiffe.Wait()
		if err != nil {
			return
		}
		s.logger.Info("Serving gRPC with SPIFFE mTLS", zap.String("spiffe_id", id.String()))
		if s.config.ServiceName != "" && !s.config.Unhealthy && !s.config.Unready {
			healthSrv.SetServingStatus(s.config.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
		}
	}()
}
