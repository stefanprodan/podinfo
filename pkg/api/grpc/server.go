// Copyright 2019 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	cachev1 "github.com/stefanprodan/podinfo/pkg/api/grpc/cache/v1"
	delayv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/delay/v1"
	echov1 "github.com/stefanprodan/podinfo/pkg/api/grpc/echo/v1"
	envv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/env/v1"
	headerv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/header/v1"
	infov1 "github.com/stefanprodan/podinfo/pkg/api/grpc/info/v1"
	panicv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/panic/v1"
	podv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/pod/v1"
	statusv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/status/v1"
	storev1 "github.com/stefanprodan/podinfo/pkg/api/grpc/store/v1"
	tokenv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/token/v1"
	versionv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/version/v1"
	"github.com/stefanprodan/podinfo/pkg/api/limits"
	"github.com/stefanprodan/podinfo/pkg/keypair"
	"github.com/stefanprodan/podinfo/pkg/spiffe"
	"github.com/stefanprodan/podinfo/pkg/store"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

// Server runs the podinfo gRPC API: the services of this package, health
// checking and reflection.
type Server struct {
	logger       *zap.Logger
	config       *Config
	spiffe       *spiffe.Source
	keyPair      *keypair.KeyPair
	cache        redis.UniversalClient
	backends     []echoBackend
	backendConns []*grpc.ClientConn
}

// Config holds the settings of the gRPC server. The mapstructure tags name
// the command flags they are read from.
type Config struct {
	Port              int           `mapstructure:"grpc-port"`
	TLS               bool          `mapstructure:"grpc-tls"`
	ServiceName       string        `mapstructure:"grpc-service-name"`
	BackendURL        []string      `mapstructure:"grpc-backend-url"`
	ClientTimeout     time.Duration `mapstructure:"grpc-client-timeout"`
	SPIFFEBackendMTLS bool          `mapstructure:"spiffe-backend-mtls"`
	SPIFFEBackendIDs  []string      `mapstructure:"spiffe-backend-ids"`
	Host              string        `mapstructure:"host"`
	Hostname          string        `mapstructure:"hostname"`
	UILogo            string        `mapstructure:"ui-logo"`
	UIMessage         string        `mapstructure:"ui-message"`
	UIColor           string        `mapstructure:"ui-color"`
	Unhealthy         bool          `mapstructure:"unhealthy"`
	Unready           bool          `mapstructure:"unready"`
	JWTSecret         string        `mapstructure:"jwt-secret"`
	DataPath          string        `mapstructure:"data-path"`
}

// NewServer returns a Server that does not listen yet. The SPIFFE source, key
// pair and cache client are optional and must be set before ListenAndServe.
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

// SetCache serves the cache service with the given client, or reports the
// cache not configured when it is nil. The caller closes it after the server stops.
func (s *Server) SetCache(client redis.UniversalClient) {
	s.cache = client
}

// ListenAndServe starts the gRPC server and returns it with its health server.
func (s *Server) ListenAndServe() (*grpc.Server, *health.Server) {
	address := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", address)
	if err != nil {
		s.logger.Fatal("failed to listen", zap.String("address", address), zap.Error(err))
	}
	return s.serve(listener)
}

func (s *Server) serve(listener net.Listener) (*grpc.Server, *health.Server) {
	creds, err := s.credentials()
	if err != nil {
		s.logger.Fatal("failed to load the gRPC TLS certificate", zap.Error(err))
	}

	if err := s.dialBackends(); err != nil {
		s.logger.Fatal("invalid gRPC backend config", zap.Error(err))
	}

	srv, healthSrv := s.newServer(grpc.Creds(creds))
	if s.spiffeTLS() {
		s.waitForSVID(healthSrv)
	}

	s.logger.Info("Starting gRPC Server",
		zap.String("addr", listener.Addr().String()),
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

	echov1.RegisterEchoServiceServer(srv, &echoServer{config: s.config, logger: s.logger, backends: s.backends})
	versionv1.RegisterVersionServiceServer(srv, &versionServer{config: s.config, logger: s.logger})
	panicv1.RegisterPanicServiceServer(srv, &panicServer{config: s.config, logger: s.logger})
	delayv1.RegisterDelayServiceServer(srv, &delayServer{config: s.config, logger: s.logger})
	headerv1.RegisterHeaderServiceServer(srv, &headerServer{config: s.config, logger: s.logger})
	infov1.RegisterInfoServiceServer(srv, &infoServer{config: s.config})
	podv1.RegisterPodServiceServer(srv, &podServer{config: s.config})
	statusv1.RegisterStatusServiceServer(srv, &statusServer{config: s.config, logger: s.logger})
	storev1.RegisterStoreServiceServer(srv, &storeServer{store: store.New(s.config.DataPath, telemetry.Tracer()), logger: s.logger})
	tokenv1.RegisterTokenServiceServer(srv, &tokenServer{config: s.config, logger: s.logger})
	envv1.RegisterEnvServiceServer(srv, &envServer{config: s.config, logger: s.logger})
	cachev1.RegisterCacheServiceServer(srv, &cacheServer{cache: s.cache, logger: s.logger})
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

// dialBackends creates the Echo client of each --grpc-backend-url.
// Connections are established on the first call.
func (s *Server) dialBackends() error {
	for i, backend := range s.config.BackendURL {
		conn, err := s.dialBackend(backend)
		if err != nil {
			// The URL is left out, it may hold credentials.
			return fmt.Errorf("invalid --grpc-backend-url at index %d: %w", i, err)
		}
		s.backendConns = append(s.backendConns, conn)
		s.backends = append(s.backends, echoBackend{url: backend, client: echov1.NewEchoServiceClient(conn)})
	}
	return nil
}

var errBackendURL = errors.New("want grpc://host:port or grpcs://host:port")

func (s *Server) dialBackend(backend string) (*grpc.ClientConn, error) {
	u, err := url.Parse(backend)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errBackendURL
	}
	if port, err := strconv.Atoi(u.Port()); err != nil || port < 1 || port > 65535 {
		return nil, errBackendURL
	}
	creds, err := s.backendCredentials(u.Scheme)
	if err != nil {
		return nil, err
	}
	// The dns scheme keeps hosts named like a resolver (dns, unix) from
	// being parsed as one.
	return grpc.NewClient("dns:///"+u.Host,
		grpc.WithTransportCredentials(creds),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithConnectParams(backendConnectParams),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(limits.MaxBackendResponseSize)))
}

// backendConnectParams caps the reconnect backoff at 5s instead of the
// default 2m, so calls recover soon after a backend comes back.
var backendConnectParams = grpc.ConnectParams{
	Backoff:           backoff.Config{BaseDelay: time.Second, Multiplier: 1.6, Jitter: 0.2, MaxDelay: 5 * time.Second},
	MinConnectTimeout: 5 * time.Second,
}

// backendCredentials returns the transport credentials for a backend URL
// scheme: plaintext for grpc, and for grpcs TLS verified against the system
// roots or SPIFFE mTLS when --spiffe-backend-mtls is set.
func (s *Server) backendCredentials(scheme string) (credentials.TransportCredentials, error) {
	switch {
	case scheme == "grpc":
		return insecure.NewCredentials(), nil
	case scheme != "grpcs":
		return nil, errBackendURL
	case !s.config.SPIFFEBackendMTLS:
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}), nil
	case s.spiffe == nil:
		return nil, errors.New("--spiffe-backend-mtls requires --spiffe-endpoint-socket")
	}
	ids, err := spiffe.ParseIDs("--spiffe-backend-ids", s.config.SPIFFEBackendIDs)
	if err != nil {
		return nil, err
	}
	s.spiffe.Start()
	return credentials.NewTLS(s.spiffe.ClientTLSConfig(ids)), nil
}

// Close closes the backend connections. Call it after the server stops.
func (s *Server) Close() error {
	errs := make([]error, 0, len(s.backendConns))
	for _, conn := range s.backendConns {
		errs = append(errs, conn.Close())
	}
	s.backendConns = nil
	return errors.Join(errs...)
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
