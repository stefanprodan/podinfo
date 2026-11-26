package signals

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
)

type Shutdown struct {
	logger                *zap.Logger
	tracerProvider        *sdktrace.TracerProvider
	loggerProvider        *sdklog.LoggerProvider
	serverShutdownTimeout time.Duration
	drainDelay            time.Duration
	beforeShutdown        func()
}

func (s *Shutdown) SetLoggerProvider(lp *sdklog.LoggerProvider) {
	s.loggerProvider = lp
}

// SetBeforeShutdown registers a function that runs as soon as the stop
// signal is received, before the health and ready states are cleared.
func (s *Shutdown) SetBeforeShutdown(fn func()) {
	s.beforeShutdown = fn
}

func NewShutdown(serverShutdownTimeout, drainDelay time.Duration, logger *zap.Logger) *Shutdown {
	return &Shutdown{
		logger:                logger,
		serverShutdownTimeout: serverShutdownTimeout,
		drainDelay:            drainDelay,
	}
}

func (s *Shutdown) Graceful(stopCh <-chan struct{}, httpServer *http.Server, httpsServer *http.Server, grpcServer *grpc.Server, grpcHealth *health.Server, healthy *int32, ready *int32) {
	// wait for SIGTERM or SIGINT
	<-stopCh

	if s.beforeShutdown != nil {
		s.beforeShutdown()
	}

	// all calls to /healthz and /readyz will fail from now on
	atomic.StoreInt32(healthy, 0)
	atomic.StoreInt32(ready, 0)
	if grpcHealth != nil {
		grpcHealth.Shutdown()
	}

	s.logger.Info("Shutting down servers", zap.Duration("drain", s.drainDelay), zap.Duration("timeout", s.serverShutdownTimeout))

	// There could be a period where a terminating pod may still receive requests. Implementing a brief wait can mitigate this.
	// See: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination
	// the readiness check interval must be lower than the drain delay
	time.Sleep(s.drainDelay)

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

	otelCtx, otelCancel := context.WithTimeout(context.Background(), s.serverShutdownTimeout)
	defer otelCancel()

	// stop OpenTelemetry tracer provider
	if s.tracerProvider != nil {
		if err := s.tracerProvider.Shutdown(otelCtx); err != nil {
			s.logger.Warn("stopping tracer provider", zap.Error(err))
		}
	}

	// stop OpenTelemetry logger provider
	if s.loggerProvider != nil {
		if err := s.loggerProvider.Shutdown(otelCtx); err != nil {
			s.logger.Warn("stopping logger provider", zap.Error(err))
		}
	}
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
