package http

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

type LoggingMiddleware struct {
	logger *zap.Logger
}

func NewLoggingMiddleware(logger *zap.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

func (m *LoggingMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields := []zap.Field{
			zap.String("proto", r.Proto),
			zap.String("uri", r.RequestURI),
			zap.String("method", r.Method),
			zap.String("remote", r.RemoteAddr),
			zap.String("user-agent", r.UserAgent()),
			telemetry.Context(r.Context()),
		}

		if id, ok := peerSPIFFEID(r); ok {
			fields = append(fields, zap.String("spiffe_id", id.String()))
		}

		m.logger.Debug(
			"request started",
			fields...,
		)
		next.ServeHTTP(w, r)
	})
}
