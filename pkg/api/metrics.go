package api

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusMiddleware struct {
	Histogram *prometheus.HistogramVec
	Counter   *prometheus.CounterVec
}

func NewPrometheusMiddleware() *PrometheusMiddleware {
	// used for monitoring and alerting (RED method)
	histogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "Seconds spent serving HTTP requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path", "status"})
	// used for horizontal pod auto-scaling (Kubernetes HPA v2)
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "The total number of HTTP requests.",
		},
		[]string{"status"},
	)

	prometheus.MustRegister(histogram)
	prometheus.MustRegister(counter)

	return &PrometheusMiddleware{
		Histogram: histogram,
		Counter:   counter,
	}
}

func (p *PrometheusMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		interceptor := &interceptor{ResponseWriter: w, statusCode: http.StatusOK}
		path := p.getRouteName(r)
		next.ServeHTTP(interceptor, r)
		var (
			status = strconv.Itoa(interceptor.statusCode)
			took   = time.Since(begin)
		)
		p.Histogram.WithLabelValues(r.Method, path, status).Observe(took.Seconds())
		p.Counter.WithLabelValues(status).Inc()
	})
}

// converts gorilla mux routes from '/api/delay/{wait}' to 'api_delay_wait'
func (p *PrometheusMiddleware) getRouteName(r *http.Request) string {
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

type interceptor struct {
	http.ResponseWriter
	statusCode int
	recorded   bool
}

func (i *interceptor) WriteHeader(code int) {
	if !i.recorded {
		i.statusCode = code
		i.recorded = true
	}
	i.ResponseWriter.WriteHeader(code)
}

func (i *interceptor) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := i.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("interceptor: can't cast parent ResponseWriter to Hijacker")
	}
	return hj.Hijack()
}
