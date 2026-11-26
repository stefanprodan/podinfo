package http

import (
	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
)

func NewOpenTelemetryMiddleware(serviceName string) mux.MiddlewareFunc {
	return otelmux.Middleware(serviceName)
}
