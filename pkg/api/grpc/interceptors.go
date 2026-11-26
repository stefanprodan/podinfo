package grpc

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/filters"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

type metrics struct {
	histogram *prometheus.HistogramVec
	counter   *prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		histogram: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: "grpc",
			Name:      "request_duration_seconds",
			Help:      "Seconds spent serving gRPC requests.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "status"}),
		counter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "grpc",
			Name:      "requests_total",
			Help:      "The total number of gRPC requests.",
		}, []string{"status"}),
	}
	reg.MustRegister(m.histogram, m.counter)
	return m
}

var defaultMetrics = sync.OnceValue(func() *metrics {
	return newMetrics(prometheus.DefaultRegisterer)
})

func (m *metrics) observe(ctx context.Context, method string, start time.Time, err error) {
	code := status.Code(err).String()
	telemetry.Observe(ctx, m.histogram.WithLabelValues(method, code), time.Since(start).Seconds())
	m.counter.WithLabelValues(code).Inc()
}

func (m *metrics) unaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	m.observe(ctx, info.FullMethod, start, err)
	return resp, err
}

func (m *metrics) streamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()
	err := handler(srv, ss)
	m.observe(ss.Context(), info.FullMethod, start, err)
	return err
}

func (s *Server) recoverPanic(ctx context.Context, method string, err *error) {
	if r := recover(); r != nil {
		s.logger.Error("gRPC handler panic",
			zap.String("method", method),
			zap.Any("panic", r),
			zap.Stack("stack"),
			telemetry.Context(ctx))
		*err = status.Error(codes.Internal, "internal server error")
	}
}

func (s *Server) unaryRecoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer s.recoverPanic(ctx, info.FullMethod, &err)
	return handler(ctx, req)
}

func (s *Server) streamRecoveryInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer s.recoverPanic(ss.Context(), info.FullMethod, &err)
	return handler(srv, ss)
}

func (s *Server) serverOptions(m *metrics) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler(otelgrpc.WithFilter(filters.None(
			filters.HealthCheck(),
			filters.ServicePrefix("grpc.reflection."),
		)))),
		grpc.ChainUnaryInterceptor(m.unaryInterceptor, s.unaryRecoveryInterceptor),
		grpc.ChainStreamInterceptor(m.streamInterceptor, s.streamRecoveryInterceptor),
	}
}
