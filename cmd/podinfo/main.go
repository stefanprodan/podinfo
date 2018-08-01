package main

import (
	"flag"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stefanprodan/k8s-podinfo/pkg/server"
	"github.com/stefanprodan/k8s-podinfo/pkg/signals"
	"github.com/stefanprodan/k8s-podinfo/pkg/version"
)

var (
	port  string
	debug bool
)

func init() {
	flag.StringVar(&port, "port", "9898", "Port to listen on.")
	flag.BoolVar(&debug, "debug", false, "sets log level to debug")
}

func main() {
	flag.Parse()

	//fscache.ReadAllFile("/Users/aleph/go/src/github.com/stefanprodan/k8s-podinfo/")

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Info().Msgf("Starting podinfo version %s commit %s", version.VERSION, version.GITCOMMIT)
	log.Debug().Msgf("Starting HTTP server on port %v", port)

	stopCh := signals.SetupSignalHandler()
	server.ListenAndServe(port, 5*time.Second, stopCh)
}
