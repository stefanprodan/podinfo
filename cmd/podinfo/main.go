package main

import (
	"context"
	"flag"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/stefanprodan/k8s-podinfo/pkg/server"
	"github.com/stefanprodan/k8s-podinfo/pkg/signals"
	"github.com/stefanprodan/k8s-podinfo/pkg/version"
)

var (
	port string
)

func init() {
	flag.StringVar(&port, "port", "8989", "Port to listen on.")
}

func main() {
	flag.Parse()

	glog.Infof("Starting podinfo version %v", version.VERSION)
	glog.Infof("Starting HTTP server on port %v", port)

	hts := &http.Server{
		Addr:    ":" + port,
		Handler: server.New(),
	}

	go func() {
		if err := hts.ListenAndServe(); err != http.ErrServerClosed {
			glog.Fatal(err)
		}
	}()
	shutdown(hts, 10*time.Second)
}

func shutdown(hs *http.Server, timeout time.Duration) {
	stopCh := signals.SetupSignalHandler()
	<-stopCh

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	glog.Infof("Shutdown with timeout: %s", timeout)

	if err := hs.Shutdown(ctx); err != nil {
		glog.Error(err)
	} else {
		glog.Info("HTTP server stopped")
	}
}
