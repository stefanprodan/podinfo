package main

import (
	"flag"
	stdlog "log"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stefanprodan/k8s-podinfo/pkg/server"
	"github.com/stefanprodan/k8s-podinfo/pkg/signals"
	"github.com/stefanprodan/k8s-podinfo/pkg/version"
)

var (
	port     string
	debug    bool
	logLevel string
)

func init() {
	flag.StringVar(&port, "port", "9898", "Port to listen on.")
	flag.BoolVar(&debug, "debug", false, "sets log level to debug")
	flag.StringVar(&logLevel, "logLevel", "debug", "sets log level as debug, info, warn, error, flat or panic ")
}

func main() {
	flag.Parse()
	setLogging()

	log.Info().Msgf("Starting podinfo version %s commit %s", version.VERSION, version.GITCOMMIT)
	log.Debug().Msgf("Starting HTTP server on port %v", port)

	stopCh := signals.SetupSignalHandler()
	server.ListenAndServe(port, 5*time.Second, stopCh)
}

func setLogging() {
	// set global log level
	switch logLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)

	}

	// keep for backwards compatibility
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	// set zerolog as standard logger
	stdlog.SetFlags(0)
	stdlog.SetOutput(log.Logger)
}
