package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

// standardMethod returns the method in upper case, or fallback when it is
// not a standard HTTP method, so client-sent methods stay a bounded set.
func standardMethod(method, fallback string) string {
	switch m := strings.ToUpper(method); m {
	case http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead, http.MethodOptions,
		http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace:
		return m
	}
	return fallback
}

func spanMethod(method string) string {
	return standardMethod(method, "HTTP")
}

func newBackendClient(transport http.RoundTripper) *http.Client {
	return &http.Client{Transport: otelhttp.NewTransport(backendURLTransport{transport},
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return spanMethod(r.Method)
		}),
	)}
}

// backendURLTransport replaces the url.full that otelhttp records on the
// client span, which keeps the query, with the backend URL as shown in logs.
type backendURLTransport struct {
	next http.RoundTripper
}

func (t backendURLTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if u, ok := displayURL(r.URL); ok {
		trace.SpanFromContext(r.Context()).SetAttributes(semconv.URLFull(u))
	}
	return t.next.RoundTrip(r)
}
