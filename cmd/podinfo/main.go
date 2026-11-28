package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stefanprodan/podinfo/pkg/api/grpc"
	"github.com/stefanprodan/podinfo/pkg/api/http"
	"github.com/stefanprodan/podinfo/pkg/keypair"
	"github.com/stefanprodan/podinfo/pkg/signals"
	"github.com/stefanprodan/podinfo/pkg/store"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
	"github.com/stefanprodan/podinfo/pkg/version"
	go_grpc "google.golang.org/grpc"
	"google.golang.org/grpc/health"
)

func main() {
	// flags definition
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.String("host", "", "Host to bind service to")
	fs.String("prefix", "/", "URL path prefix for HTTP routes")
	fs.Int("port", 9898, "HTTP port to bind service to")
	fs.Int("secure-port", 0, "HTTPS port")
	fs.Int("port-metrics", 0, "metrics port")
	fs.Int("grpc-port", 0, "gRPC port")
	fs.String("grpc-service-name", "podinfo", "gPRC service name")
	fs.Bool("grpc-tls", false, "serve the gRPC port with TLS using the --cert-path certificate, or with SPIFFE mTLS when --spiffe-endpoint-socket is set")
	fs.StringSlice("grpc-backend-url", []string{}, "gRPC backend service URL, grpc://host:port or grpcs://host:port for TLS")
	fs.Duration("grpc-client-timeout", 2*time.Minute, "gRPC backend call timeout duration")
	fs.String("level", "info", "log level debug, info, warn, error, fatal or panic")
	fs.StringSlice("backend-url", []string{}, "backend service URL")
	fs.Duration("http-client-timeout", 2*time.Minute, "client timeout duration")
	fs.Duration("http-server-timeout", 30*time.Second, "server read and write timeout duration")
	fs.Duration("server-shutdown-timeout", 5*time.Second, "server graceful shutdown timeout duration")
	fs.String("data-path", "/data", "data local path")
	fs.String("config-path", "", "config dir path")
	fs.String("cert-path", "/data/cert", "certificate path for the HTTPS port and the gRPC port with --grpc-tls")
	fs.String("spiffe-endpoint-socket", "", "SPIFFE Workload API address, e.g. unix:///spiffe-workload-api/spire-agent.sock; when set, the HTTPS port uses SPIFFE mTLS instead of --cert-path")
	fs.StringSlice("spiffe-authorized-ids", []string{}, "SPIFFE IDs allowed to connect to the HTTPS port; when empty, any ID in the server's trust domain is allowed")
	fs.Bool("spiffe-backend-mtls", false, "call https and grpcs backends with SPIFFE mTLS; requires --spiffe-endpoint-socket")
	fs.StringSlice("spiffe-backend-ids", []string{}, "SPIFFE IDs accepted from https and grpcs backends; requires --spiffe-backend-mtls; when empty, any ID in the server's trust domain is accepted")
	fs.String("artifact-path", "./kustomize", "directory served as a tar.gz at /artifact; empty disables the route")
	fs.String("config", "config.yaml", "config file name")
	fs.String("ui-path", "", "deprecated, directory with an index.html page that replaces the embedded UI")
	fs.String("ui-logo", "", "UI logo URL, defaults to the embedded image")
	fs.String("ui-color", "#1f4e8c", "UI color")
	fs.String("ui-message", "greetings from podinfo", "UI message")
	fs.Bool("h2c", false, "allow upgrading to H2C")
	fs.Bool("random-delay", false, "between 0 and 5 seconds random delay by default")
	fs.String("random-delay-unit", "s", "either s(seconds) or ms(milliseconds")
	fs.Int("random-delay-min", 0, "min for random delay: 0 by default")
	fs.Int("random-delay-max", 5, "max for random delay: 5 by default")
	fs.Bool("random-error", false, "1/3 chances of a random response error")
	fs.Bool("unhealthy", false, "when set, healthy state is never reached")
	fs.Bool("unready", false, "when set, ready state is never reached")
	fs.Int("stress-cpu", 0, "number of CPU cores with 100 load")
	fs.Int("stress-memory", 0, "MB of data to load into memory")
	fs.String("cache-server", "", "Redis or Valkey URL in the format 'redis://[user:password@]host:port[/db]', rediss:// for TLS, valkey:// and valkeys:// as aliases, more cluster nodes or Sentinels as '?addr=host:port' parameters")
	fs.Bool("cache-cluster", false, "connect to the cache in cluster mode, implied when --cache-server has more than one address")
	fs.String("cache-sentinel-primary", "", "name of the primary monitored by Sentinel (what Redis calls the master name); the --cache-server addresses are then the Sentinels")
	fs.String("cache-ca-file", "", "PEM CA bundle that verifies the cache server certificate instead of the system roots")
	fs.String("cache-cert-path", "", "directory with the tls.crt and tls.key client key pair presented to the cache server, a rotated pair is used for new connections; requires a rediss:// or valkeys:// --cache-server")
	fs.String("otel-service-name", "", "deprecated, use OTEL_SERVICE_NAME")

	versionFlag := fs.BoolP("version", "v", false, "get version number")

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

	// bind flags and environment variables
	viper.BindPFlags(fs)
	viper.RegisterAlias("backendUrl", "backend-url")
	hostname, _ := os.Hostname()
	viper.SetDefault("jwt-secret", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
	viper.SetDefault("cache-username", "")
	viper.SetDefault("cache-password", "")
	viper.SetDefault("cache-sentinel-password", "")
	viper.Set("hostname", hostname)
	viper.Set("version", version.VERSION)
	viper.Set("revision", version.REVISION)
	viper.SetEnvPrefix("PODINFO")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// load config from file
	if _, fileErr := os.Stat(filepath.Join(viper.GetString("config-path"), viper.GetString("config"))); fileErr == nil {
		viper.SetConfigName(strings.Split(viper.GetString("config"), ".")[0])
		viper.AddConfigPath(viper.GetString("config-path"))
		if readErr := viper.ReadInConfig(); readErr != nil {
			fmt.Printf("Error reading config file, %v\n", readErr)
		}
	}

	if viper.GetString("ui-logo") == "" {
		viper.Set("ui-logo", path.Join("/", viper.GetString("prefix"), "ui/cuddle_clap.gif"))
	}

	// configure logging
	level, err := parseLevel(viper.GetString("level"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %s\n", err.Error())
		os.Exit(1)
	}
	stderrLogger := initZap(level, nil)
	defer stderrLogger.Sync()

	// configure OpenTelemetry from the OTEL_* environment variables
	otelServiceName := viper.GetString("otel-service-name")
	if otelServiceName != "" {
		stderrLogger.Warn("--otel-service-name is deprecated, use OTEL_SERVICE_NAME")
	}
	if viper.GetString("ui-path") != "" {
		stderrLogger.Warn("--ui-path is deprecated, the UI is embedded in the binary")
	}
	tel, err := telemetry.Setup(context.Background(), otelServiceName, stderrLogger)
	if err != nil {
		stderrLogger.Error("OpenTelemetry setup failed, telemetry disabled", zap.Error(err))
	}
	shutdownTimeout := viper.GetDuration("server-shutdown-timeout")
	defer shutdownTelemetry(tel, shutdownTimeout, stderrLogger)

	logger := withTelemetry(level, tel, shutdownTimeout, os.Exit)
	stdLog := zap.RedirectStdLog(logger)
	defer stdLog()

	// start stress tests if any
	beginStressTest(viper.GetInt("stress-cpu"), viper.GetInt("stress-memory"), logger)

	resetInvalidPorts(fs, "port", "secure-port", "grpc-port", "port-metrics")

	// validate random delay options
	if viper.GetInt("random-delay-min") < 0 || viper.GetInt("random-delay-max") < 0 {
		logger.Panic("`--random-delay-min` and `--random-delay-max` should not be negative")
	}
	if viper.GetInt("random-delay-max") < viper.GetInt("random-delay-min") {
		logger.Panic("`--random-delay-max` should be greater than `--random-delay-min`")
	}

	switch delayUnit := viper.GetString("random-delay-unit"); delayUnit {
	case
		"s",
		"ms":
		break
	default:
		logger.Panic("`random-delay-unit` accepted values are: s|ms")
	}

	// load gRPC server config
	var grpcCfg grpc.Config
	if err := viper.Unmarshal(&grpcCfg); err != nil {
		logger.Panic("config unmarshal failed", zap.Error(err))
	}

	// load HTTP server config
	var srvCfg http.Config
	if err := viper.Unmarshal(&srvCfg); err != nil {
		logger.Panic("config unmarshal failed", zap.Error(err))
	}

	// log version and port
	logger.Info("Starting podinfo",
		zap.String("version", viper.GetString("version")),
		zap.String("revision", viper.GetString("revision")),
		zap.String("port", srvCfg.Port),
	)

	if err := store.Sweep(srvCfg.DataPath); err != nil {
		logger.Warn("removing the temporary store files failed", zap.Error(err), zap.String("path", srvCfg.DataPath))
	}

	// start HTTP server
	srv, err := http.NewServer(&srvCfg, logger)
	if err != nil {
		logger.Fatal("invalid HTTP server config", zap.Error(err))
	}

	var keyPair *keypair.KeyPair
	if srv.SPIFFESource() == nil && (srvCfg.SecurePort != "0" || (grpcCfg.Port > 0 && grpcCfg.TLS)) {
		keyPair, err = keypair.New(srvCfg.CertPath, logger)
		if err != nil {
			logger.Fatal("loading the TLS key pair failed", zap.Error(err), zap.String("path", srvCfg.CertPath))
		}
		srv.SetKeyPair(keyPair)
	}

	// start gRPC server
	var grpcSrv *grpc.Server
	var grpcServer *go_grpc.Server
	var grpcHealth *health.Server
	if grpcCfg.Port == 0 && len(grpcCfg.BackendURL) > 0 {
		logger.Fatal("--grpc-backend-url requires --grpc-port")
	}
	if grpcCfg.Port == 0 && grpcCfg.TLS {
		logger.Fatal("--grpc-tls requires --grpc-port")
	}
	if grpcCfg.Port > 0 {
		grpcSrv = grpc.NewServer(&grpcCfg, logger)
		grpcSrv.SetSPIFFESource(srv.SPIFFESource())
		grpcSrv.SetKeyPair(keyPair)
		grpcSrv.SetCache(srv.Cache())
		grpcServer, grpcHealth = grpcSrv.ListenAndServe()
	}

	httpServer, httpsServer, healthy, ready := srv.ListenAndServe()

	// graceful shutdown
	stopCh := signals.SetupSignalHandler()
	drainDelay := 3 * time.Second
	if viper.GetString("level") == "debug" {
		drainDelay = 0
	}
	sd := signals.NewShutdown(srvCfg.ServerShutdownTimeout, drainDelay, logger)
	sd.SetBeforeShutdown(srv.Stop)
	sd.Graceful(stopCh, httpServer, httpsServer, grpcServer, grpcHealth, healthy, ready)
	if grpcSrv != nil {
		if err := grpcSrv.Close(); err != nil {
			logger.Warn("closing the gRPC backend connections failed", zap.Error(err))
		}
	}
	if err := srv.Close(); err != nil {
		logger.Warn("releasing HTTP server resources failed", zap.Error(err))
	}
	if keyPair != nil {
		if err := keyPair.Close(); err != nil {
			logger.Warn("closing the TLS key pair watcher failed", zap.Error(err))
		}
	}
}

// resetInvalidPorts restores the flag value of the ports that are not
// integers, as Kubernetes service links set variables such as
// PODINFO_GRPC_PORT=tcp://10.0.0.1:9999 for a Service named podinfo-grpc.
func resetInvalidPorts(fs *pflag.FlagSet, names ...string) {
	for _, name := range names {
		if _, err := strconv.Atoi(viper.GetString(name)); err != nil {
			port, _ := fs.GetInt(name)
			viper.Set(name, strconv.Itoa(port))
		}
	}
}

func parseLevel(logLevel string) (zap.AtomicLevel, error) {
	var level zap.AtomicLevel
	switch logLevel {
	case "debug":
		level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	case "fatal":
		level = zap.NewAtomicLevelAt(zapcore.FatalLevel)
	case "panic":
		level = zap.NewAtomicLevelAt(zapcore.PanicLevel)
	default:
		return level, fmt.Errorf("unknown log level %q", logLevel)
	}
	return level, nil
}

func initZap(level zap.AtomicLevel, loggerProvider *sdklog.LoggerProvider) *zap.Logger {
	zapEncoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	cores := []zapcore.Core{
		telemetry.NewTraceIDCore(zapcore.NewCore(zapcore.NewJSONEncoder(zapEncoderConfig), zapcore.Lock(os.Stderr), level)),
	}
	if loggerProvider != nil {
		cores = append(cores, otelzap.NewCore("github.com/stefanprodan/podinfo",
			otelzap.WithLoggerProvider(loggerProvider),
		))
	}
	return zap.New(
		zapcore.NewSamplerWithOptions(zapcore.NewTee(cores...), time.Second, 100, 100),
		zap.ErrorOutput(zapcore.Lock(os.Stderr)),
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.IncreaseLevel(level),
	)
}

func withTelemetry(level zap.AtomicLevel, tel *telemetry.Telemetry, shutdownTimeout time.Duration, exit func(int)) *zap.Logger {
	return initZap(level, tel.LoggerProvider()).WithOptions(zap.WithFatalHook(tel.ExitHook(shutdownTimeout, exit)))
}

func shutdownTelemetry(tel *telemetry.Telemetry, timeout time.Duration, logger *zap.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := tel.Shutdown(ctx); err != nil {
		logger.Warn("OpenTelemetry shutdown failed", zap.Error(err))
	}
}

var stressMemoryPayload []byte

func beginStressTest(cpus int, mem int, logger *zap.Logger) {
	if cpus > 0 {
		logger.Info("starting CPU stress", zap.Int("cores", cpus))
		for i := 0; i < cpus; i++ {
			go func() {
				for {
					runtime.Gosched()
				}
			}()
		}
	}

	if mem > 0 {
		path := "/tmp/podinfo.data"
		f, err := os.Create(path)

		if err != nil {
			logger.Error("memory stress failed", zap.Error(err))
		}

		if err := f.Truncate(1000000 * int64(mem)); err != nil {
			logger.Error("memory stress failed", zap.Error(err))
		}

		stressMemoryPayload, err = os.ReadFile(path)
		f.Close()
		os.Remove(path)
		if err != nil {
			logger.Error("memory stress failed", zap.Error(err))
		}
		logger.Info("starting MEMORY stress", zap.Int("memory", len(stressMemoryPayload)))
	}
}
