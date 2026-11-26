package http

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

func (s *Server) instrument(next http.Handler) http.Handler {
	untraced := map[string]bool{
		s.prefixedPath("/healthz"): true,
		s.prefixedPath("/readyz"):  true,
		s.prefixedPath("/metrics"): true,
	}
	return otelhttp.NewHandler(next, "podinfo",
		otelhttp.WithServerName(s.config.Hostname),
		otelhttp.WithFilter(func(r *http.Request) bool {
			return !untraced[r.URL.Path]
		}),
	)
}

func routeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route, err := mux.CurrentRoute(r).GetPathTemplate(); err == nil {
			attr := semconv.HTTPRoute(route)
			span := trace.SpanFromContext(r.Context())
			span.SetName(spanMethod(r.Method) + " " + route)
			span.SetAttributes(attr)
			if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
				labeler.Add(attr)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func spanMethod(method string) string {
	switch m := strings.ToUpper(method); m {
	case http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead, http.MethodOptions,
		http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace:
		return m
	}
	return "HTTP"
}

func newBackendClient(transport http.RoundTripper) *http.Client {
	return &http.Client{Transport: otelhttp.NewTransport(transport)}
}

func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
