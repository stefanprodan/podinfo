package http

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

type prometheusMiddleware struct {
	histogram  *prometheus.HistogramVec
	counter    *prometheus.CounterVec
	registerer prometheus.Registerer
	gatherer   prometheus.Gatherer
}

func newPrometheusMiddleware(reg prometheus.Registerer, g prometheus.Gatherer) *prometheusMiddleware {
	p := &prometheusMiddleware{
		registerer: reg,
		gatherer:   g,
		// used for monitoring and alerting (RED method)
		histogram: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Seconds spent serving HTTP requests.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),
		// used for horizontal pod auto-scaling (Kubernetes HPA v2)
		counter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "The total number of HTTP requests.",
		}, []string{"status"}),
	}
	reg.MustRegister(p.histogram, p.counter)
	return p
}

var defaultPrometheusMiddleware = sync.OnceValue(func() *prometheusMiddleware {
	return newPrometheusMiddleware(prometheus.DefaultRegisterer, prometheus.DefaultGatherer)
})

// Metrics godoc
// @Summary Prometheus metrics
// @Description returns HTTP requests duration and Go runtime metrics
// @Tags Kubernetes
// @Produce plain
// @Router /metrics [get]
// @Success 200 {string} string "OK"
func (p *prometheusMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := p.getRouteName(r)
		m := httpsnoop.CaptureMetrics(next, w, r)
		status := strconv.Itoa(m.Code)
		telemetry.Observe(r.Context(), p.histogram.WithLabelValues(standardMethod(r.Method, "other"), path, status), m.Duration.Seconds())
		p.counter.WithLabelValues(status).Inc()
	})
}

// converts gorilla mux routes from '/api/delay/{wait}' to 'api_delay_wait'
func (p *prometheusMiddleware) getRouteName(r *http.Request) string {
	if mux.CurrentRoute(r) != nil {
		if name := mux.CurrentRoute(r).GetName(); len(name) > 0 {
			return urlToLabel(name)
		}
		if path, err := mux.CurrentRoute(r).GetPathTemplate(); err == nil {
			if len(path) > 0 {
				return urlToLabel(path)
			}
		}
	}
	return urlToLabel(r.RequestURI)
}

var invalidChars = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// converts a URL path to a string compatible with Prometheus label value.
func urlToLabel(path string) string {
	result := invalidChars.ReplaceAllString(path, "_")
	result = strings.ToLower(strings.Trim(result, "_"))
	if result == "" {
		result = "root"
	}
	return result
}

func (p *prometheusMiddleware) metricsHandler() http.Handler {
	return promhttp.InstrumentMetricHandler(
		p.registerer,
		promhttp.HandlerFor(p.gatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}),
	)
}
