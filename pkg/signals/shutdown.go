// Copyright 2022 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package signals

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
)

// Shutdown stops the HTTP and gRPC servers the way a Kubernetes rollout needs:
// it fails the probes, waits for the endpoint removal to propagate, then
// drains the in-flight requests.
type Shutdown struct {
	logger                *zap.Logger
	serverShutdownTimeout time.Duration
	drainDelay            time.Duration
	beforeShutdown        func()
	sleep                 func(time.Duration)
}

// SetBeforeShutdown registers a function that runs as soon as the stop
// signal is received, before the health and ready states are cleared.
func (s *Shutdown) SetBeforeShutdown(fn func()) {
	s.beforeShutdown = fn
}

// NewShutdown returns a Shutdown that waits drainDelay after failing the
// probes and then gives the servers serverShutdownTimeout to drain.
func NewShutdown(serverShutdownTimeout, drainDelay time.Duration, logger *zap.Logger) *Shutdown {
	return &Shutdown{
		logger:                logger,
		serverShutdownTimeout: serverShutdownTimeout,
		drainDelay:            drainDelay,
		sleep:                 time.Sleep,
	}
}

// Graceful blocks until stopCh is closed, then fails the health checks, waits
// for the drain delay and shuts the servers down within the shutdown timeout.
func (s *Shutdown) Graceful(stopCh <-chan struct{}, httpServer *http.Server, httpsServer *http.Server, grpcServer *grpc.Server, grpcHealth *health.Server, healthy *int32, ready *int32) {
	<-stopCh

	if s.beforeShutdown != nil {
		s.beforeShutdown()
	}

	// /healthz and /readyz fail from here on.
	atomic.StoreInt32(healthy, 0)
	atomic.StoreInt32(ready, 0)
	if grpcHealth != nil {
		grpcHealth.Shutdown()
	}

	s.logger.Info("Shutting down servers", zap.Duration("drain", s.drainDelay), zap.Duration("timeout", s.serverShutdownTimeout))

	// A terminating pod may still receive requests while its endpoint removal
	// propagates. The readiness probe period must be shorter than the drain delay.
	// See https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination
	s.sleep(s.drainDelay)

	ctx, cancel := context.WithTimeout(context.Background(), s.serverShutdownTimeout)
	defer cancel()

	var wg sync.WaitGroup
	if grpcServer != nil {
		wg.Go(func() {
			s.stopGRPC(ctx, grpcServer)
		})
	}
	if httpServer != nil {
		wg.Go(func() {
			if err := httpServer.Shutdown(ctx); err != nil {
				s.logger.Warn("HTTP server graceful shutdown failed", zap.Error(err))
			}
		})
	}
	if httpsServer != nil {
		wg.Go(func() {
			if err := httpsServer.Shutdown(ctx); err != nil {
				s.logger.Warn("HTTPS server graceful shutdown failed", zap.Error(err))
			}
		})
	}
	wg.Wait()
}

func (s *Shutdown) stopGRPC(ctx context.Context, grpcServer *grpc.Server) {
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-ctx.Done():
		s.logger.Warn("GRPC server graceful shutdown timed out")
		grpcServer.Stop()
		<-stopped
	}
}
