package main

import (
	"flag"
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

	glog.Infof("Starting podinfo version %s commit %s", version.VERSION, version.GITCOMMIT)
	glog.Infof("Starting HTTP server on port %v", port)

	stopCh := signals.SetupSignalHandler()
	server.ListenAndServe(port, 5*time.Second, stopCh)
}
