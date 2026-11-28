// Copyright 2018 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

// LoggingMiddleware logs every request at debug level before it is handled,
// with the SPIFFE ID of the client when it presented one.
type LoggingMiddleware struct {
	logger *zap.Logger
}

// NewLoggingMiddleware returns a LoggingMiddleware that writes to logger.
func NewLoggingMiddleware(logger *zap.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

// Handler returns next wrapped with the request log.
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
