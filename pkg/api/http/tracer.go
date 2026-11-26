package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/felixge/httpsnoop"
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
		span := trace.SpanFromContext(r.Context())
		labeler, _ := otelhttp.LabelerFromContext(r.Context())
		if tpl, err := mux.CurrentRoute(r).GetPathTemplate(); err == nil {
			route := routeTemplate(tpl)
			span.SetName(spanMethod(r.Method) + " " + route)
			span.SetAttributes(semconv.HTTPRoute(route))
			labeler.Add(semconv.HTTPRoute(route))
		}
		m := httpsnoop.CaptureMetrics(next, w, r)
		if m.Code >= http.StatusInternalServerError {
			errorType := semconv.ErrorTypeKey.String(strconv.Itoa(m.Code))
			span.SetAttributes(errorType)
			labeler.Add(errorType)
		}
	})
}

// routeTemplate strips the variable patterns from a mux path template.
func routeTemplate(tpl string) string {
	var b strings.Builder
	var depth int
	var pattern bool
	for _, c := range tpl {
		switch {
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				pattern = false
			}
		case c == ':' && depth == 1:
			pattern = true
		}
		if !pattern {
			b.WriteRune(c)
		}
	}
	return b.String()
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
	return &http.Client{Transport: otelhttp.NewTransport(transport,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return spanMethod(r.Method)
		}),
	)}
}

func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
