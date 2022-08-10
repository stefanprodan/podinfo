package grpc

import (
	"errors"
	"fmt"
	"net"

	user "github.com/SimifiniiCTO/simfiny-microservice-template/api-definition/gen"
	"github.com/SimifiniiCTO/simfiny-microservice-template/pkg/database"
	"github.com/SimifiniiCTO/simfiny-microservice-template/pkg/metrics"
	"github.com/dtm-labs/dtm/dtmgrpc/dtmgimp"
	"github.com/dtm-labs/dtm/dtmgrpc/dtmgpb"
	"github.com/newrelic/go-agent/v3/newrelic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	user.UnimplementedServiceServer
	config                *Config
	Logger                *zap.Logger
	InstrumentationClient *newrelic.Application
	DatabaseConn          *database.Db
	DtmManagerClient      dtmgpb.DtmClient
	MetricEngine          *metrics.MetricsEngine // *core_metrics_newrelic.ServiceMetricsEngine
	ServiceMetrics        *metrics.ServiceMetrics
}

type Config struct {
	Port            int    `mapstructure:"GRPC_PORT"`
	ServiceName     string `mapstructure:"GRPC_SERVICE_NAME"`
	NewRelicLicense string `mapstructure:"NEWRELIC_KEY"`
	RpcDeadline     int    `mapstructure:"GRPC_DEADLINE_IN_MS"`
	RpcRetries      int    `mapstructure:"GRPC_RETRIES"`
	RpcRetryTimeout int    `mapstructure:"GRPC_RETRY_TIMEOUT"`
	RpcRetryBackoff int    `mapstructure:"GRPC_RETRY_BACKOOF"`
	DtmManagerURI   string `mapstructure:"DTX_MANAGER_URI"`
}

var _ user.ServiceServer = (*Server)(nil)

type ServiceConnInterface func(cc grpc.ClientConnInterface) interface{}

type ServiceOperation string
type ServiceDtxOperation string

// RegisterGrpcServer registers the grpc server object
func (server *Server) RegisterGrpcServer(srv *grpc.Server) {
	user.RegisterServiceServer(srv, server)
}

// NewServer returns a new instance of the grpc server
func NewServer(config *Config, logger *zap.Logger, instrumentationSdk *newrelic.Application, db *database.Db, svcMetricsEngine *metrics.MetricsEngine) (*Server, error) {
	dtmClient, err := connectToDtmManager(&config.DtmManagerURI)
	if err != nil {
		logger.Error("failed to connect to dtm-manager service", zap.Error(err))
		return nil, err
	}

	logger.Info(fmt.Sprintf("successfully established connection to dtm manager service. URI: %s", config.DtmManagerURI))

	srv := &Server{
		Logger:                logger,
		config:                config,
		InstrumentationClient: instrumentationSdk,
		DatabaseConn:          db,
		DtmManagerClient:      *dtmClient,
		MetricEngine:          svcMetricsEngine,
		ServiceMetrics:        svcMetricsEngine.Metrics,
	}

	return srv, nil
}

func (s *Server) ListenAndServe() {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", s.config.Port))
	if err != nil {
		s.Logger.Fatal("failed to listen", zap.Int("port", s.config.Port))
	}

	srv := grpc.NewServer()
	server := health.NewServer()
	reflection.Register(srv)
	grpc_health_v1.RegisterHealthServer(srv, server)
	server.SetServingStatus(s.config.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)

	if err := srv.Serve(listener); err != nil {
		s.Logger.Fatal("failed to serve", zap.Error(err))
	}
}

func connectToDtmManager(connURI *string) (*dtmgpb.DtmClient, error) {
	var f ServiceConnInterface
	var c interface{}
	var err error

	if connURI == nil {
		return nil, errors.New("invalid input argument. invalid connection string provided")
	}

	f = func(cc grpc.ClientConnInterface) interface{} {
		return dtmgpb.NewDtmClient(cc)
	}

	c, err = connect(*connURI, f)
	if err != nil {
		return nil, err
	}

	client, ok := c.(dtmgpb.DtmClient)
	if !ok {
		return nil, errors.New("failed to cast to financial integration service client")
	}

	return &client, nil
}

func connect(connURI string, f ServiceConnInterface) (interface{}, error) {
	conn, err := establishServiceConn(connURI)
	if err != nil {
		return nil, err
	}

	return f(conn), nil
}

func establishServiceConn(connURI string) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(), grpc.WithUnaryInterceptor(dtmgimp.GrpcClientLog))
	conn, err := grpc.Dial(connURI, opts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
