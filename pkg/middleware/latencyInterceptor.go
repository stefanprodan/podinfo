package middleware

import (
	"context"
	"time"

	"github.com/SimifiniiCTO/simfiny-microservice-template/pkg/metrics"
	rkgrpcctx "github.com/rookie-ninja/rk-grpc/interceptor/context"
	rkgrpcmid "github.com/rookie-ninja/rk-grpc/v2/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// RequestLatencyUnaryServerInterceptor Create new unary server interceptor to capture request latency.
func RequestLatencyUnaryServerInterceptor(e *metrics.MetricsEngine, m *metrics.ServiceMetrics) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		var opStatusCode codes.Code = codes.OK
		start := time.Now()

		ctx = rkgrpcmid.WrapContextForServer(ctx)
		method, path, _, _ := rkgrpcmid.GetGwInfo(rkgrpcctx.GetIncomingHeaders(ctx))
		resp, err := handler(ctx, req)

		if err != nil {
			opStatusCode = codes.Internal
		}

		e.RecordLatencyMetric(m.DbOperationLatency, method, path, opStatusCode, time.Since(start))
		return resp, err
	}
}

// RequestLatencyStreamServerInterceptor Create new stream server interceptor to capture request latency.
func RequestLatencyStreamServerInterceptor(e *metrics.MetricsEngine, m *metrics.ServiceMetrics) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Before invoking
		wrappedStream := rkgrpcctx.WrapServerStream(stream)
		wrappedStream.WrappedContext = rkgrpcmid.WrapContextForServer(wrappedStream.WrappedContext)

		var opStatusCode codes.Code = codes.OK
		start := time.Now()

		method, path, _, _ := rkgrpcmid.GetGwInfo(rkgrpcctx.GetIncomingHeaders(wrappedStream.WrappedContext))
		err := handler(srv, wrappedStream)

		if err != nil {
			opStatusCode = codes.Internal
		}

		e.RecordLatencyMetric(m.DbOperationLatency, method, path, opStatusCode, time.Since(start))
		return err
	}
}
