package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace/noop"
)

func durationHistogram(t *testing.T, path string) (uint64, []map[string]string) {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var count uint64
	var exemplars []map[string]string
	for _, mf := range mfs {
		if mf.GetName() != "http_request_duration_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			var match bool
			for _, l := range m.GetLabel() {
				match = match || l.GetName() == "path" && l.GetValue() == path
			}
			if !match {
				continue
			}
			count += m.GetHistogram().GetSampleCount()
			for _, b := range m.GetHistogram().GetBucket() {
				if e := b.GetExemplar(); e != nil {
					labels := map[string]string{}
					for _, l := range e.GetLabel() {
						labels[l.GetName()] = l.GetValue()
					}
					exemplars = append(exemplars, labels)
				}
			}
		}
	}
	return count, exemplars
}

func serve(srv *Server, path string, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header = header
	rr := httptest.NewRecorder()
	srv.handler.ServeHTTP(rr, req)
	return rr
}

func TestMetricsPathLabel(t *testing.T) {
	srv := newInstrumentedServer("/")
	for path, label := range map[string]string{
		"/env":        "env",
		"/status/418": "status",
		"/delay/0":    "delay",
		"/chunked/0":  "chunked_wait_0_9",
		"/store/abc":  "store",
		"/api/echo/x": "api_echo",
	} {
		before, _ := durationHistogram(t, label)
		serve(srv, path, http.Header{})
		if after, _ := durationHistogram(t, label); after != before+1 {
			t.Errorf("%s: http_request_duration_seconds{path=%q} count %d, want %d", path, label, after, before+1)
		}
	}
}

func TestMetricsExemplars(t *testing.T) {
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	spans := setupTracing(t)
	reader := setupMetrics(t)

	srv := newInstrumentedServer("/")
	serve(srv, "/version", http.Header{"Traceparent": {"00-" + traceID + "-00f067aa0ba902b7-01"}})
	serve(srv, "/headers", http.Header{"Traceparent": {"00-" + traceID + "-00f067aa0ba902b7-00"}})

	otel.SetTracerProvider(noop.NewTracerProvider())
	serve(newInstrumentedServer("/"), "/api/info", http.Header{"Traceparent": {"00-" + traceID + "-00f067aa0ba902b7-01"}})

	t.Run("sampled span", func(t *testing.T) {
		ended := spans.GetSpans()
		if len(ended) != 1 {
			t.Fatalf("got %d spans, want 1", len(ended))
		}
		spanID := ended[0].SpanContext.SpanID().String()
		_, exemplars := durationHistogram(t, "version")
		var found bool
		for _, e := range exemplars {
			found = found || e["span_id"] == spanID && e["trace_id"] == traceID
		}
		if !found {
			t.Errorf("no exemplar with trace_id %s and span_id %s in %v", traceID, spanID, exemplars)
		}
	})

	for _, tt := range []struct{ name, path string }{
		{name: "unsampled span", path: "headers"},
		{name: "remote span without tracer provider", path: "api_info"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			count, exemplars := durationHistogram(t, tt.path)
			if count == 0 {
				t.Error("request not observed")
			}
			if len(exemplars) != 0 {
				t.Errorf("got exemplars %v, want none", exemplars)
			}
		})
	}

	t.Run("openmetrics", func(t *testing.T) {
		rr := serve(srv, "/metrics", http.Header{"Accept": {"application/openmetrics-text"}})
		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/openmetrics-text") {
			t.Errorf("Content-Type: got %q", ct)
		}
		body, _ := io.ReadAll(rr.Body)
		if !strings.Contains(string(body), `trace_id="`+traceID+`"`) {
			t.Error("exemplar not exposed")
		}
	})

	t.Run("text", func(t *testing.T) {
		rr := serve(srv, "/metrics", http.Header{})
		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
			t.Errorf("Content-Type: got %q", ct)
		}
		body, _ := io.ReadAll(rr.Body)
		if !strings.Contains(string(body), "http_request_duration_seconds_bucket") {
			t.Error("histogram not exposed")
		}
		if strings.Contains(string(body), "trace_id") {
			t.Error("exemplar exposed in the text format")
		}
	})

	t.Run("otel metrics", func(t *testing.T) {
		var rm metricdata.ResourceMetrics
		if err := reader.Collect(context.Background(), &rm); err != nil {
			t.Fatal(err)
		}
		var found bool
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				found = found || m.Name == "http.server.request.duration"
			}
		}
		if !found {
			t.Error("http.server.request.duration not recorded")
		}
	})
}

func methodCount(t *testing.T, method, path string) uint64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var count uint64
	for _, mf := range mfs {
		if mf.GetName() != "http_request_duration_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["method"] == method && labels["path"] == path {
				count += m.GetHistogram().GetSampleCount()
			}
		}
	}
	return count
}

func TestMethodLabel(t *testing.T) {
	for _, m := range []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace,
	} {
		if got := methodLabel(m); got != m {
			t.Errorf("methodLabel(%q) = %q, want %q", m, got, m)
		}
	}
	for _, m := range []string{"FOO", "get", "", "PROPFIND"} {
		if got := methodLabel(m); got != "other" {
			t.Errorf("methodLabel(%q) = %q, want other", m, got)
		}
	}
}

func TestMetricsMethodLabel(t *testing.T) {
	srv := newInstrumentedServer("/")
	before := methodCount(t, "other", "echo")
	req := httptest.NewRequest("FOO", "/echo", nil)
	srv.handler.ServeHTTP(httptest.NewRecorder(), req)
	if after := methodCount(t, "other", "echo"); after != before+1 {
		t.Errorf(`http_request_duration_seconds{method="other",path="echo"} count %d, want %d`, after, before+1)
	}
	if n := methodCount(t, "FOO", "echo"); n != 0 {
		t.Errorf(`http_request_duration_seconds{method="FOO"} count %d, want 0`, n)
	}
}
