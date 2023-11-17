package http

import (
	"bufio"
	"fmt"
	"io"
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

// Metrics godoc
// @Summary Prometheus metrics
// @Description returns HTTP requests duration and Go runtime metrics
// @Tags Kubernetes
// @Produce plain
// @Router /metrics [get]
// @Success 200 {string} string "OK"
func (p *PrometheusMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		interceptor := &interceptor{ResponseWriter: w, statusCode: http.StatusOK}
		path := p.getRouteName(r)
		next.ServeHTTP(interceptor.wrappedResponseWriter(), r)
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

func (i *interceptor) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := i.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("interceptor: can't cast parent ResponseWriter to Hijacker")
	}
	return hj.Hijack()
}

func (i *interceptor) WriteHeader(code int) {
	if !i.recorded {
		i.statusCode = code
		i.recorded = true
	}
	i.ResponseWriter.WriteHeader(code)
}

// Returns a wrapped http.ResponseWriter that implements the same optional interfaces
// that the underlying ResponseWriter has.
// Handle every possible combination so that code that checks for the existence of each
// optional interface functions properly.
// Based on https://github.com/felixge/httpsnoop/blob/eadd4fad6aac69ae62379194fe0219f3dbc80fd3/wrap_generated_gteq_1.8.go#L66
func (i *interceptor) wrappedResponseWriter() http.ResponseWriter {
	closeNotifier, isCloseNotifier := i.ResponseWriter.(http.CloseNotifier)
	flush, isFlusher := i.ResponseWriter.(http.Flusher)
	hijack, isHijacker := i.ResponseWriter.(http.Hijacker)
	push, isPusher := i.ResponseWriter.(http.Pusher)
	readFrom, isReaderFrom := i.ResponseWriter.(io.ReaderFrom)

	switch {
	case !isCloseNotifier && !isFlusher && !isHijacker && !isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
		}{i}

	case isCloseNotifier && !isFlusher && !isHijacker && !isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
		}{i, closeNotifier}

	case !isCloseNotifier && isFlusher && !isHijacker && !isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Flusher
		}{i, flush}

	case !isCloseNotifier && !isFlusher && isHijacker && !isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Hijacker
		}{i, hijack}

	case !isCloseNotifier && !isFlusher && !isHijacker && isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Pusher
		}{i, push}

	case !isCloseNotifier && !isFlusher && !isHijacker && !isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			io.ReaderFrom
		}{i, readFrom}

	case isCloseNotifier && isFlusher && !isHijacker && !isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Flusher
		}{i, closeNotifier, flush}

	case isCloseNotifier && !isFlusher && isHijacker && !isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Hijacker
		}{i, closeNotifier, hijack}

	case isCloseNotifier && !isFlusher && !isHijacker && isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Pusher
		}{i, closeNotifier, push}

	case isCloseNotifier && !isFlusher && !isHijacker && !isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			io.ReaderFrom
		}{i, closeNotifier, readFrom}

	case !isCloseNotifier && isFlusher && isHijacker && !isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Flusher
			http.Hijacker
		}{i, flush, hijack}

	case !isCloseNotifier && isFlusher && !isHijacker && isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Flusher
			http.Pusher
		}{i, flush, push}

	case !isCloseNotifier && isFlusher && !isHijacker && !isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Flusher
			io.ReaderFrom
		}{i, flush, readFrom}

	case !isCloseNotifier && !isFlusher && isHijacker && isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Hijacker
			http.Pusher
		}{i, hijack, push}

	case !isCloseNotifier && !isFlusher && isHijacker && !isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Hijacker
			io.ReaderFrom
		}{i, hijack, readFrom}

	case !isCloseNotifier && !isFlusher && !isHijacker && isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Pusher
			io.ReaderFrom
		}{i, push, readFrom}

	case isCloseNotifier && isFlusher && isHijacker && !isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Flusher
			http.Hijacker
		}{i, closeNotifier, flush, hijack}

	case isCloseNotifier && isFlusher && !isHijacker && isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Flusher
			http.Pusher
		}{i, closeNotifier, flush, push}

	case isCloseNotifier && isFlusher && !isHijacker && !isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Flusher
			io.ReaderFrom
		}{i, closeNotifier, flush, readFrom}

	case isCloseNotifier && !isFlusher && isHijacker && isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Hijacker
			http.Pusher
		}{i, closeNotifier, hijack, push}

	case isCloseNotifier && !isFlusher && isHijacker && !isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Hijacker
			io.ReaderFrom
		}{i, closeNotifier, hijack, readFrom}

	case isCloseNotifier && !isFlusher && !isHijacker && isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Pusher
			io.ReaderFrom
		}{i, closeNotifier, push, readFrom}

	case !isCloseNotifier && isFlusher && isHijacker && isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Flusher
			http.Hijacker
			http.Pusher
		}{i, flush, hijack, push}

	case !isCloseNotifier && isFlusher && isHijacker && !isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Flusher
			http.Hijacker
			io.ReaderFrom
		}{i, flush, hijack, readFrom}

	case !isCloseNotifier && isFlusher && !isHijacker && isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Flusher
			http.Pusher
			io.ReaderFrom
		}{i, flush, push, readFrom}

	case !isCloseNotifier && !isFlusher && isHijacker && isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Hijacker
			http.Pusher
			io.ReaderFrom
		}{i, hijack, push, readFrom}

	case isCloseNotifier && isFlusher && isHijacker && isPusher && !isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Flusher
			http.Hijacker
			http.Pusher
		}{i, closeNotifier, flush, hijack, push}

	case isCloseNotifier && isFlusher && isHijacker && !isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Flusher
			http.Hijacker
			io.ReaderFrom
		}{i, closeNotifier, flush, hijack, readFrom}

	case isCloseNotifier && isFlusher && !isHijacker && isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Flusher
			http.Pusher
			io.ReaderFrom
		}{i, closeNotifier, flush, push, readFrom}

	case isCloseNotifier && !isFlusher && isHijacker && isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Hijacker
			http.Pusher
			io.ReaderFrom
		}{i, closeNotifier, hijack, push, readFrom}

	case !isCloseNotifier && isFlusher && isHijacker && isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.Flusher
			http.Hijacker
			http.Pusher
			io.ReaderFrom
		}{i, flush, hijack, push, readFrom}

	case isCloseNotifier && isFlusher && isHijacker && isPusher && isReaderFrom:
		return struct {
			http.ResponseWriter
			http.CloseNotifier
			http.Flusher
			http.Hijacker
			http.Pusher
			io.ReaderFrom
		}{i, closeNotifier, flush, hijack, push, readFrom}

	default:
		return struct {
			http.ResponseWriter
		}{i}
	}
}
