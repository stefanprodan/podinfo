package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "net/http/pprof"

	user "github.com/SimifiniiCTO/simfiny-microservice-template/api-definition/gen"
	"github.com/SimifiniiCTO/simfiny-microservice-template/pkg/database"
	"github.com/SimifiniiCTO/simfiny-microservice-template/pkg/metrics"
	"github.com/SimifiniiCTO/simfiny-microservice-template/pkg/middleware"
	"github.com/SimifiniiCTO/simfiny-microservice-template/pkg/version"

	rpc "github.com/SimifiniiCTO/simfiny-microservice-template/pkg/grpc"
	"github.com/labstack/gommon/log"
	"github.com/newrelic/go-agent/v3/integrations/nrzap"
	"github.com/newrelic/go-agent/v3/newrelic"
	rkboot "github.com/rookie-ninja/rk-boot"
	rkgrpc "github.com/rookie-ninja/rk-grpc/boot"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Application entrance.
func main() {
	// flags definition
	fs := initFlags()

	ctx := context.Background()
	// capture goroutines waiting on synchronization primitives
	runtime.SetBlockProfileRate(1)
	versionFlag := fs.BoolP("version", "v", false, "get version number")

	ParseFlags(fs, versionFlag)
	LoadEnvVariables(fs)

	// Create a new boot instance.
	boot := rkboot.NewBoot()

	// get logger entry defined in boot config file
	logger := boot.GetZapLoggerEntry("zap-logger").Logger

	// configure new relic sdk
	var newrelicLicenseKey = viper.GetString("NEWRELIC_KEY")
	var serviceName = viper.GetString("GRPC_SERVICE_NAME")
	instrumentationSdk, err := initNewRelicInstrumentationSDK(newrelicLicenseKey, serviceName, logger)
	if err != nil {
		logger.Panic(err.Error())
	}

	var version = viper.GetString("VERSION")
	var docs = viper.GetString("SERVICE_DOCUMENTATION")
	var environment = viper.GetString("SERVICE_ENVIRONMENT")
	var pointOfContact = viper.GetString("POINT_OF_CONTACT")
	var metricsReportingEnabled = viper.GetBool("METRICS_REPORTING_ENABLED")

	// initialize service metrics
	metricEngine, err := initServiceMetricEngine(newrelicLicenseKey,
		serviceName, version, docs, pointOfContact, environment, logger, metricsReportingEnabled)
	if err != nil {
		logger.Panic(err.Error())
	}

	// validate port
	if _, err := strconv.Atoi(viper.GetString("GRPC_PORT")); err != nil {
		port, _ := fs.GetInt("GRPC_PORT")
		viper.Set("GRPC_PORT", strconv.Itoa(port))
	}

	// validate random delay options
	if viper.GetInt("RANDOM_DELAY_MAX") < viper.GetInt("RANDOM_DELAY_MIN") {
		logger.Panic("`--RANDOM_DELAY_MAX` should be greater than `--RANDOM_DELAY_MIN`")
	}

	switch delayUnit := viper.GetString("RANDOM_DELAY_UNIT"); delayUnit {
	case
		"s",
		"ms":
		break
	default:
		logger.Panic("`RANDOM_DELAY_UNIT` accepted values are: s|ms")
	}

	// initialize database connection
	db, err := initDatabaseConn(ctx, logger, instrumentationSdk)
	if err != nil {
		logger.Panic(err.Error())
	}

	// load gRPC server config
	var grpcCfg rpc.Config
	if err := viper.Unmarshal(&grpcCfg); err != nil {
		logger.Panic("config unmarshal failed", zap.Error(err))
	}

	// initiate new instance of server
	srv, err := rpc.NewServer(&grpcCfg, logger, instrumentationSdk, db, metricEngine)
	if err != nil {
		logger.Panic(err.Error())
	}

	// Get grpc entry with name
	grpcEntry := boot.GetEntry("service").(*rkgrpc.GrpcEntry)
	grpcEntry.AddUnaryInterceptors(middleware.RequestLatencyUnaryServerInterceptor(srv.MetricEngine, srv.ServiceMetrics))
	grpcEntry.AddStreamInterceptors(middleware.RequestLatencyStreamServerInterceptor(srv.MetricEngine, srv.ServiceMetrics))

	grpcEntry.AddRegFuncGrpc(srv.RegisterGrpcServer)
	grpcEntry.AddRegFuncGw(user.RegisterServiceHandlerFromEndpoint)

	// Bootstrap
	boot.Bootstrap(context.Background())

	// Wait for shutdown sig
	boot.WaitForShutdownSig(context.Background())
}

// LoadEnvVariables binds a set of flags to and loads environment variables
func LoadEnvVariables(fs *pflag.FlagSet) {
	viper.AddConfigPath("/go/src/github.com/SimifiniiCTO/simfiny-microservice-template")
	viper.BindPFlags(fs)
	viper.RegisterAlias("BACKEND_SERVICE_URLS", "BACKEND_URL")
	viper.SetConfigName("service")
	viper.SetConfigType("env")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	hostname, _ := os.Hostname()
	viper.SetDefault("JWT_SECRET", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
	viper.SetDefault("UI_LOGO", "https://raw.githubusercontent.com/stefanprodan/podinfo/gh-pages/cuddle_clap.gif")
	viper.Set("HOSTNAME", hostname)
	viper.Set("VERSION", version.VERSION)
	viper.Set("REVISION", version.REVISION)
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		log.Error(err.Error())
		return
	}
}

// ParseFlags parses a set of defined flags
func ParseFlags(fs *pflag.FlagSet, versionFlag *bool) {
	// parse flags
	err := fs.Parse(os.Args[1:])
	switch {
	case err == pflag.ErrHelp:
		os.Exit(0)
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err.Error())
		fs.PrintDefaults()
		os.Exit(2)
	case *versionFlag:
		fmt.Println(version.VERSION)
		os.Exit(0)
	}
}

// initFlags env flags
func initFlags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.String("HOST", "", "Host to bind service to")
	fs.Int("METRICS_PORT", 0, "metrics port")
	fs.Int("GRPC_PORT", 9896, "gRPC port")
	fs.String("GRPC_SERVICE_NAME", "service", "gPRC service name")
	fs.String("LOG_LEVEL", "info", "log level debug, info, warn, error, flat or panic")
	fs.StringSlice("BACKEND_URL", []string{}, "backend service URL")
	fs.String("CONFIG_PATH", "", "config dir path")
	fs.String("CONFIG_FILE", "config.yaml", "config file name")
	fs.Bool("RANDOM_DELAY", false, "between 0 and 5 seconds random delay by default")
	fs.String("RANDOM_DELAY_UNIT", "s", "either s(seconds) or ms(milliseconds")
	fs.Int("RANDOM_DELAY_MIN", 0, "min for random delay: 0 by default")
	fs.Int("RANDOM_DELAY_MAX", 5, "max for random delay: 5 by default")
	fs.Bool("RANDOM_ERROR", false, "1/3 chances of a random response error")
	fs.Int("STRESS_CPU", 0, "number of CPU cores with 100 load")
	fs.Int("STRESS_MEMORY", 0, "MB of data to load into memory")

	fs.String("NEWRELIC_KEY", "62fd721c712d5863a4e75b8f547b7c1ea884NRAL", "new relic license key")

	fs.String("DATABASE_HOST", "service_db", "database host string")
	fs.Int("DATABASE_PORT", 5432, "database port")
	fs.String("DATABASE_USER", "service_db", "database user string")
	fs.String("DATABASE_PASSWORD", "service_db", "database password string")
	fs.String("DATABASE_NAME", "service_db", "database name")
	fs.String("DATABASE_SSLMODE", "disable", "wether to establish a tls connection with the database")
	fs.Int("MAX_DATABASE_CONNECTION_ATTEMPTS", 2, "max database connection attempts")
	fs.Int("MAX_DATABASE_CONNECTION_RETRIES", 2, "max database connection attempts")
	fs.Duration("MAX_DATABASE_RETRY_TIMEOUT", 500*time.Millisecond, "max time until a db connection request is seen as timing out")
	fs.Duration("MAX_DATABASE_RETRY_SLEEP", 100*time.Millisecond, "max time to sleep in between db connection attempts")
	fs.Int("GRPC_DEADLINE_IN_MS", 2000, "RPC operation deadline in ms")
	fs.Int("GRPC_RETRIES", 3, "RPC operation deadline in ms")
	fs.Int("GRPC_RETRY_TIMEOUT", 600, "RPC operation retry timeout in ms")
	fs.Int("GRPC_RETRY_BACKOOF", 600, "RPC operation retry backoff in ms")

	fs.String("DTX_MANAGER_URI", "dtm:36790", "uri of the dtm manager service")

	fs.String("SERVICE_ENVIRONMENT", "dev", "environment in which service is running")
	fs.String("VERSION", "1.0.0", "version of service actively running")
	fs.String("SERVICE_DOCUMENTATION", "https://github.com/SimifiniiCTO/simfinii/blob/main/src/backend/services/user-service/documentation/setup.md", "location of service docs")
	fs.String("POINT_OF_CONTACT", "yoanyomba", "service point of contact")
	fs.Bool("METRICS_REPORTING_ENABLED", true, "enable metrics reporting")

	return fs
}

// initDatabaseConn initializes database connection
func initDatabaseConn(ctx context.Context, logger *zap.Logger, instrumentationSdk *newrelic.Application) (*database.Db, error) {
	host := viper.GetString("DATABASE_HOST")
	port := viper.GetInt("DATABASE_PORT")
	user := viper.GetString("DATABASE_USER")
	password := viper.GetString("DATABASE_PASSWORD")
	dbname := viper.GetString("DATABASE_NAME")
	sslmode := viper.GetString("DATABASE_SSLMODE")

	maxDBConnAttempts := viper.GetInt("MAX_DATABASE_CONNECTION_ATTEMPTS")
	maxRetriesPerDBConnectionAttempt := viper.GetInt("MAX_DATABASE_CONNECTION_RETRIES")
	maxDBRetryTimeout := viper.GetDuration("MAX_DATABASE_RETRY_TIMEOUT")
	maxDBSleepInterval := viper.GetDuration("MAX_DATABASE_RETRY_SLEEP")

	// initialize a newrelic txn to trace the db connection event
	txn := instrumentationSdk.StartTransaction("database-connection")
	defer txn.End()

	initializationParams := &database.ConnectionInitializationParams{
		ConnectionParams: &database.ConnectionParameters{
			Host:         host,
			User:         user,
			Password:     password,
			DatabaseName: dbname,
			Port:         port,
			SslMode:      sslmode,
		},
		Logger:                 logger,
		MaxConnectionAttempts:  maxDBConnAttempts,
		MaxRetriesPerOperation: maxRetriesPerDBConnectionAttempt,
		RetryTimeOut:           maxDBRetryTimeout,
		RetrySleepInterval:     maxDBSleepInterval,
		Telemetry:              instrumentationSdk,
	}

	db, err := database.New(ctx, initializationParams)
	if err != nil {
		return nil, err
	}

	logger.Info("successfully initialized database connection object")
	return db, nil
}

func initServiceMetricEngine(newrelicLicenseKey, serviceName, version, docs, pointOfContact, environment string, logger *zap.Logger, metricsReportingEnabled bool) (*metrics.MetricsEngine, error) {
	if newrelicLicenseKey != "" {
		if logger == nil {
			return nil, errors.New("invalid input argument. logger cannot be nil")
		}

		details := &metrics.ServiceDetails{
			ServiceName:        serviceName,
			Version:            version,
			PointOfContact:     pointOfContact,
			DocumentationLink:  docs,
			Environment:        environment,
			NewRelicLicenseKey: newrelicLicenseKey,
		}

		return metrics.NewMetricsEngine(details, logger, metricsReportingEnabled)
	}

	return nil, errors.New(fmt.Sprintf("invalid input parameter. param: newrelicLicenseKey = %s", newrelicLicenseKey))
}

// initNewRelicInstrumentationSDK configures the new relic sdk with metadata specific to this service
func initNewRelicInstrumentationSDK(newrelicLicenseKey string, serviceName string, logger *zap.Logger) (*newrelic.Application, error) {
	if newrelicLicenseKey != "" {
		return newrelic.NewApplication(
			newrelic.ConfigAppName(serviceName),
			newrelic.ConfigLicense(newrelicLicenseKey),
			func(cfg *newrelic.Config) {
				cfg.ErrorCollector.RecordPanics = true
				cfg.ErrorCollector.Enabled = true
				cfg.TransactionEvents.Enabled = true
				cfg.Enabled = true
				cfg.TransactionEvents.Enabled = true
				cfg.Attributes.Enabled = true
				cfg.BrowserMonitoring.Enabled = true
				cfg.TransactionTracer.Enabled = true
				cfg.SpanEvents.Enabled = true
				cfg.RuntimeSampler.Enabled = true
				cfg.DistributedTracer.Enabled = true
				cfg.AppName = serviceName
				cfg.BrowserMonitoring.Enabled = true
				cfg.CustomInsightsEvents.Enabled = true
				cfg.DatastoreTracer.InstanceReporting.Enabled = true
				cfg.DatastoreTracer.QueryParameters.Enabled = true
				cfg.DatastoreTracer.DatabaseNameReporting.Enabled = true
				cfg.Logger = nrzap.Transform(logger)
			},
			// Use nrzap to register the logger with the agent:
			nrzap.ConfigLogger(logger.Named("newrelic")),
			newrelic.ConfigDistributedTracerEnabled(true),
			newrelic.ConfigEnabled(true),
		)
	}

	return nil, errors.New(fmt.Sprintf("invalid input parameter. param: newrelicLicenseKey = %s", newrelicLicenseKey))
}
