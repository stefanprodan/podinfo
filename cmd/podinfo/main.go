package main

import (
	"flag"
	"io/ioutil"
	stdlog "log"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stefanprodan/k8s-podinfo/pkg/server"
	"github.com/stefanprodan/k8s-podinfo/pkg/signals"
	"github.com/stefanprodan/k8s-podinfo/pkg/version"
)

var (
	port                string
	debug               bool
	logLevel            string
	stressCPU           int
	stressMemory        int
	stressMemoryPayload []byte
)

func init() {
	flag.StringVar(&port, "port", "9898", "Port to listen on.")
	flag.BoolVar(&debug, "debug", false, "sets log level to debug")
	flag.StringVar(&logLevel, "logLevel", "debug", "sets log level as debug, info, warn, error, flat or panic ")
	flag.IntVar(&stressCPU, "stressCPU", 0, "Number of CPU cores with 100% load")
	flag.IntVar(&stressMemory, "stressMemory", 0, "MB of data to load into memory")
}

func main() {
	flag.Parse()
	setLogging()

	log.Info().Msgf("Starting podinfo version %s commit %s", version.VERSION, version.GITCOMMIT)
	log.Debug().Msgf("Starting HTTP server on port %v", port)

	stopCh := signals.SetupSignalHandler()
	beginStressTest(stressCPU, stressMemory)
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

func beginStressTest(cpus int, mem int) {
	done := make(chan int)
	if cpus > 0 {
		log.Info().Msgf("Starting CPU stress, %v core(s)", cpus)
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
			log.Error().Err(err).Msgf("Memory stress failed")
		}

		if err := f.Truncate(1000000 * int64(mem)); err != nil {
			log.Error().Err(err).Msgf("Memory stress failed")
		}

		stressMemoryPayload, err = ioutil.ReadFile(path)
		f.Close()
		os.Remove(path)
		if err != nil {
			log.Error().Err(err).Msgf("Memory stress failed")
		}
		log.Info().Msgf("Starting memory stress, size %v", len(stressMemoryPayload))
	}
}
