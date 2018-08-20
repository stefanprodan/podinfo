package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stefanprodan/k8s-podinfo/pkg/api"
	"github.com/stefanprodan/k8s-podinfo/pkg/signals"
	"github.com/stefanprodan/k8s-podinfo/pkg/version"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// flags definition
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.Int("port", 9898, "port")
	fs.String("backend-url", "", "backend service URL")
	fs.Duration("http-client-timeout", 2*time.Minute, "client timeout duration")
	fs.Duration("http-server-timeout", 30*time.Second, "server read and write timeout duration")
	fs.Duration("http-server-shutdown-timeout", 5*time.Second, "server graceful shutdown timeout duration")
	fs.String("data-path", "/data", "data local path")
	fs.String("config-path", "", "config local path")
	fs.String("ui-path", "./ui", "UI local path")
	fs.String("ui-color", "blue", "UI color")
	fs.String("ui-message", fmt.Sprintf("greetings from podinfo v%v", version.VERSION), "UI message")
	fs.Int("stress-cpu", 0, "Number of CPU cores with 100 load")
	fs.Int("stress-memory", 0, "MB of data to load into memory")
	versionFlag := fs.Bool("version", false, "get version number")

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
	viper.Set("hostname", hostname)
	viper.Set("version", version.VERSION)
	viper.Set("revision", version.REVISION)
	viper.SetEnvPrefix("PI")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// configure logging
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
	zapConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json",
		EncoderConfig:    zapEncoderConfig,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, _ := zapConfig.Build()
	defer logger.Sync()

	// log version and port
	logger.Info("Starting podinfo",
		zap.String("version", viper.GetString("version")),
		zap.String("revision", viper.GetString("revision")),
		zap.String("port", viper.GetString("port")),
	)

	// start stress test
	beginStressTest(viper.GetInt("stress-cpu"), viper.GetInt("stress-memory"), logger)

	// configure API
	srvCfg := &api.Config{
		Port:                      viper.GetString("port"),
		Hostname:                  viper.GetString("hostname"),
		HttpServerShutdownTimeout: viper.GetDuration("http-server-shutdown-timeout"),
		HttpServerTimeout:         viper.GetDuration("http-server-timeout"),
		BackendURL:                viper.GetString("backend-url"),
		ConfigPath:                viper.GetString("config-path"),
		DataPath:                  viper.GetString("data-path"),
		HttpClientTimeout:         viper.GetDuration("http-client-timeout"),
		UIColor:                   viper.GetString("ui-color"),
		UIPath:                    viper.GetString("ui-path"),
		UIMessage:                 viper.GetString("ui-message"),
	}

	// start HTTP server
	srv, _ := api.NewServer(srvCfg, logger)
	stopCh := signals.SetupSignalHandler()
	srv.ListenAndServe(stopCh)
}

var stressMemoryPayload []byte

func beginStressTest(cpus int, mem int, logger *zap.Logger) {
	done := make(chan int)
	if cpus > 0 {
		logger.Info("starting CPU stress", zap.Int("cores", cpus))
		for i := 0; i < cpus; i++ {
			go func() {
				for {
					select {
					case <-done:
						return
					default:

					}
				}
			}()
		}
	}

	if mem > 0 {
		path := "/tmp/podinfo.data"
		f, err := os.Create(path)

		if err != nil {
			log.Error().Err(err).Msgf("memory stress failed")
		}

		if err := f.Truncate(1000000 * int64(mem)); err != nil {
			logger.Error("memory stress failed", zap.Error(err))
		}

		stressMemoryPayload, err = ioutil.ReadFile(path)
		f.Close()
		os.Remove(path)
		if err != nil {
			logger.Error("memory stress failed", zap.Error(err))
		}
		logger.Info("starting CPU stress", zap.Int("memory", len(stressMemoryPayload)))
	}
}
