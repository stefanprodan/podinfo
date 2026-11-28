package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/store"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

const (
	testTraceID     = "4bf92f3577b34da6a3ce929d0e0e4736"
	testParentID    = "00f067aa0ba902b7"
	testTraceparent = "00-" + testTraceID + "-" + testParentID + "-01"
)

func newInstrumentedServer(prefix string) *Server {
	srv := NewMockServer()
	srv.config.Prefix = prefix
	srv.setupHandler()
	return srv
}

func setupTracing(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	spans := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithSyncer(spans),
	)
	setGlobalTracing(t, tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return spans
}

func setGlobalTracing(t *testing.T, tp trace.TracerProvider) {
	t.Helper()
	prevTP, prevP := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevP)
	})
}

func setupMetrics(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	t.Cleanup(func() {
		otel.SetMeterProvider(prev)
		_ = mp.Shutdown(context.Background())
	})
	return reader
}

func attr(attrs []attribute.KeyValue, key attribute.Key) (attribute.Value, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value, true
		}
	}
	return attribute.Value{}, false
}

func spansOfKind(spans tracetest.SpanStubs, kind trace.SpanKind) tracetest.SpanStubs {
	var out tracetest.SpanStubs
	for _, s := range spans {
		if s.SpanKind == kind {
			out = append(out, s)
		}
	}
	return out
}

func TestServerSpan(t *testing.T) {
	tests := []struct {
		prefix    string
		method    string
		path      string
		wantName  string
		route     string
		code      int64
		errorType string
	}{
		{prefix: "/", method: "GET", path: "/env", wantName: "GET /env", route: "/env", code: 200},
		{prefix: "/", method: "GET", path: "/status/418", wantName: "GET /status/{code}", route: "/status/{code}", code: 418},
		{prefix: "/", method: "GET", path: "/status/503", wantName: "GET /status/{code}", route: "/status/{code}", code: 503, errorType: "503"},
		{prefix: "/", method: "GET", path: "/chunked/0", wantName: "GET /chunked/{wait}", route: "/chunked/{wait}", code: 200},
		{prefix: "/", method: "GET", path: "/unknown/path", wantName: "GET", code: 404},
		{prefix: "/", method: "POST", path: "/version", wantName: "POST", code: 405},
		{prefix: "/", method: "get", path: "/echo", wantName: "GET /echo", route: "/echo", code: 202},
		{prefix: "/", method: "GET", path: "/healthz"},
		{prefix: "/", method: "GET", path: "/readyz"},
		{prefix: "/", method: "GET", path: "/metrics"},
		{prefix: "/podinfo", method: "GET", path: "/podinfo/env", wantName: "GET /podinfo/env", route: "/podinfo/env", code: 200},
		{prefix: "/podinfo", method: "GET", path: "/podinfo/healthz"},
		{prefix: "/podinfo", method: "GET", path: "/podinfo/readyz"},
		{prefix: "/podinfo", method: "GET", path: "/podinfo/metrics"},
	}
	for _, tt := range tests {
		t.Run(tt.prefix+" "+tt.method+" "+tt.path, func(t *testing.T) {
			spans := setupTracing(t)
			reader := setupMetrics(t)
			srv := newInstrumentedServer(tt.prefix)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			srv.handler.ServeHTTP(httptest.NewRecorder(), req)

			ended := spans.GetSpans()
			if tt.wantName == "" {
				if len(ended) != 0 {
					t.Fatalf("got spans %v, want none", ended)
				}
				return
			}
			if len(ended) != 1 {
				t.Fatalf("got %d spans, want 1", len(ended))
			}
			span := ended[0]
			if span.Name != tt.wantName {
				t.Errorf("span name %q, want %q", span.Name, tt.wantName)
			}
			if span.SpanKind != trace.SpanKindServer {
				t.Errorf("span kind %v, want %v", span.SpanKind, trace.SpanKindServer)
			}
			if code, _ := attr(span.Attributes, "http.response.status_code"); code.AsInt64() != tt.code {
				t.Errorf("http.response.status_code %v, want %d", code.AsInt64(), tt.code)
			}
			route, ok := attr(span.Attributes, "http.route")
			if tt.route == "" && ok || route.AsString() != tt.route {
				t.Errorf("http.route %q, want %q", route.AsString(), tt.route)
			}
			errorType, ok := attr(span.Attributes, "error.type")
			if tt.errorType == "" && ok || errorType.AsString() != tt.errorType {
				t.Errorf("error.type %q, want %q", errorType.AsString(), tt.errorType)
			}

			var rm metricdata.ResourceMetrics
			if err := reader.Collect(context.Background(), &rm); err != nil {
				t.Fatal(err)
			}
			var found bool
			for _, sm := range rm.ScopeMetrics {
				for _, m := range sm.Metrics {
					if m.Name != "http.server.request.duration" {
						continue
					}
					for _, dp := range m.Data.(metricdata.Histogram[float64]).DataPoints {
						found = true
						route, ok := dp.Attributes.Value("http.route")
						if tt.route == "" && ok || route.AsString() != tt.route {
							t.Errorf("metric http.route %q, want %q", route.AsString(), tt.route)
						}
						errorType, ok := dp.Attributes.Value("error.type")
						if tt.errorType == "" && ok || errorType.AsString() != tt.errorType {
							t.Errorf("metric error.type %q, want %q", errorType.AsString(), tt.errorType)
						}
					}
				}
			}
			if !found {
				t.Error("http.server.request.duration not recorded")
			}
		})
	}
}

func TestRouteTemplate(t *testing.T) {
	for tpl, want := range map[string]string{
		"/env":                          "/env",
		"/store/{hash}":                 "/store/{hash}",
		"/status/{code:[0-9]+}":         "/status/{code}",
		"/{a:[a-z]{2,3}}/b/{c:x:y}/d:e": "/{a}/b/{c}/d:e",
	} {
		if got := routeTemplate(tpl); got != want {
			t.Errorf("routeTemplate(%q) = %q, want %q", tpl, got, want)
		}
	}
}

func TestServerAddress(t *testing.T) {
	spans := setupTracing(t)
	reader := setupMetrics(t)
	srv := newInstrumentedServer("/")

	req := httptest.NewRequest("GET", "/version", nil)
	req.Host = "attacker.example"
	srv.handler.ServeHTTP(httptest.NewRecorder(), req)

	ended := spans.GetSpans()
	if len(ended) != 1 {
		t.Fatalf("got %d spans, want 1", len(ended))
	}
	if got, _ := attr(ended[0].Attributes, "server.address"); got.AsString() != srv.config.Hostname {
		t.Errorf("span server.address %q, want %q", got.AsString(), srv.config.Hostname)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "http.server.request.duration" {
				continue
			}
			for _, dp := range m.Data.(metricdata.Histogram[float64]).DataPoints {
				found = true
				if got, _ := dp.Attributes.Value("server.address"); got.AsString() != srv.config.Hostname {
					t.Errorf("metric server.address %q, want %q", got.AsString(), srv.config.Hostname)
				}
			}
		}
	}
	if !found {
		t.Error("http.server.request.duration not recorded")
	}
}

func TestErrorResponseStatus(t *testing.T) {
	large := func() io.Reader { return bytes.NewReader(make([]byte, maxRequestBodySize+1)) }
	tests := []struct {
		method  string
		path    string
		body    io.Reader
		code    int
		message string
	}{
		{method: "POST", path: "/cache/key", code: http.StatusServiceUnavailable, message: "cache server is offline"},
		{method: "GET", path: "/cache/key", code: http.StatusServiceUnavailable, message: "cache server is offline"},
		{method: "DELETE", path: "/cache/key", code: http.StatusServiceUnavailable, message: "cache server is offline"},
		{method: "GET", path: "/token/validate", code: http.StatusUnauthorized, message: "authorization bearer header required"},
		{method: "POST", path: "/token", body: large(), code: http.StatusRequestEntityTooLarge, message: "request body too large"},
		{method: "POST", path: "/echo", body: large(), code: http.StatusRequestEntityTooLarge, message: "request body too large"},
		{method: "GET", path: "/store/invalid", code: http.StatusBadRequest, message: "invalid hash"},
		{method: "GET", path: "/store/" + store.Hash([]byte("missing")), code: http.StatusNotFound, message: "file not found"},
		{method: "POST", path: "/store", body: strings.NewReader("data"), code: http.StatusInternalServerError, message: "writing file failed"},
		{method: "GET", path: "/status/99999999999999999999", code: http.StatusBadRequest, message: "invalid status code"},
		{method: "GET", path: "/status/99", code: http.StatusBadRequest, message: "invalid status code"},
		{method: "GET", path: "/status/1000", code: http.StatusBadRequest, message: "invalid status code"},
		{method: "GET", path: "/delay/99999999999999999999", code: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			spans := setupTracing(t)
			srv := newInstrumentedServer("/")
			srv.store = store.New(filepath.Join(t.TempDir(), "missing"), telemetry.Tracer())

			if tt.body == nil {
				tt.body = http.NoBody
			}
			rr := httptest.NewRecorder()
			srv.handler.ServeHTTP(rr, httptest.NewRequest(tt.method, tt.path, tt.body))

			if rr.Code != tt.code {
				t.Errorf("status %d, want %d", rr.Code, tt.code)
			}
			var resp struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("invalid error JSON %q: %v", rr.Body.String(), err)
			}
			if resp.Code != tt.code || tt.message != "" && resp.Message != tt.message {
				t.Errorf("body %+v, want code %d and message %q", resp, tt.code, tt.message)
			}

			servers := spansOfKind(spans.GetSpans(), trace.SpanKindServer)
			if len(servers) != 1 {
				t.Fatalf("got %d server spans, want 1", len(servers))
			}
			span := servers[0]
			var exceptions []string
			for _, e := range span.Events {
				if msg, ok := attr(e.Attributes, "exception.message"); e.Name == "exception" && ok {
					exceptions = append(exceptions, msg.AsString())
				}
			}
			if tt.code >= http.StatusInternalServerError {
				if span.Status.Code != codes.Error {
					t.Errorf("span status %v, want %v", span.Status.Code, codes.Error)
				}
				if len(exceptions) != 1 || exceptions[0] != tt.message {
					t.Errorf("exceptions %q, want %q", exceptions, tt.message)
				}
				return
			}
			if span.Status.Code != codes.Unset {
				t.Errorf("span status %v, want %v", span.Status.Code, codes.Unset)
			}
			if len(exceptions) != 0 {
				t.Errorf("exceptions %q, want none", exceptions)
			}
		})
	}
}

func TestStoreSpans(t *testing.T) {
	spans := setupTracing(t)
	srv := newInstrumentedServer("/")
	dataDir := t.TempDir()
	srv.store = store.New(dataDir, telemetry.Tracer())

	srv.handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/store", strings.NewReader("data")))
	srv.handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/store/"+store.Hash([]byte("data")), nil))
	srv.handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/store/"+store.Hash([]byte("missing")), nil))

	ended := spans.GetSpans()
	servers := spansOfKind(ended, trace.SpanKindServer)
	internals := spansOfKind(ended, trace.SpanKindInternal)
	if len(servers) != 3 || len(internals) != 3 {
		t.Fatalf("got %d server and %d internal spans, want 3 each", len(servers), len(internals))
	}
	for i, want := range []struct {
		name   string
		status codes.Code
	}{
		{name: "store write"},
		{name: "store read"},
		{name: "store read", status: codes.Error},
	} {
		span := internals[i]
		if span.Name != want.name || span.Status.Code != want.status {
			t.Errorf("span %d: %q %v, want %q %v", i, span.Name, span.Status.Code, want.name, want.status)
		}
		if span.Parent.SpanID() != servers[i].SpanContext.SpanID() {
			t.Errorf("span %q is not a child of the server span", span.Name)
		}
		if file, _ := attr(span.Attributes, "file.path"); !strings.HasPrefix(file.AsString(), dataDir) {
			t.Errorf("file.path %q", file.AsString())
		}
	}
}

type recordedRequest struct {
	header http.Header
}

func newRecordingBackend(t *testing.T) (*httptest.Server, func() []recordedRequest) {
	t.Helper()
	var mu sync.Mutex
	var requests []recordedRequest
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, recordedRequest{header: r.Header.Clone()})
		mu.Unlock()
		w.Write([]byte(`"backend"`))
	}))
	t.Cleanup(backend.Close)
	return backend, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return requests
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func echoWithTraceHeaders(srv *Server) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/echo", strings.NewReader("hello"))
	req.Header.Set("traceparent", testTraceparent)
	req.Header.Set("x-request-id", "req-1")
	req.Header.Set("x-b3-traceid", testTraceID)
	req.Header.Set("x-b3-spanid", testParentID)
	req.Header.Set("x-b3-sampled", "1")
	req.Header.Set("x-ot-span-context", "stale")
	rr := httptest.NewRecorder()
	srv.handler.ServeHTTP(rr, req)
	return rr
}

func assertForwardedHeaders(t *testing.T, h http.Header) {
	t.Helper()
	if got := h.Get("x-request-id"); got != "req-1" {
		t.Errorf("x-request-id %q, want %q", got, "req-1")
	}
	for _, name := range []string{"x-b3-traceid", "x-b3-spanid", "x-b3-sampled", "x-ot-span-context"} {
		if got := h.Get(name); got != "" {
			t.Errorf("%s forwarded: %q", name, got)
		}
	}
}

func TestEchoPropagation(t *testing.T) {
	spans := setupTracing(t)
	backend, requests := newRecordingBackend(t)
	srv := newInstrumentedServer("/")
	srv.config.BackendURL = []string{backend.URL}
	var calls atomic.Int32
	srv.backend = newBackendClient(roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return http.DefaultTransport.RoundTrip(r)
	}))

	echoWithTraceHeaders(srv)
	echoWithTraceHeaders(srv)

	if n := calls.Load(); n != 2 {
		t.Errorf("shared backend client got %d calls, want 2", n)
	}
	got := requests()
	if len(got) != 2 {
		t.Fatalf("backend got %d requests, want 2", len(got))
	}
	ended := spans.GetSpans()
	servers := spansOfKind(ended, trace.SpanKindServer)
	clients := spansOfKind(ended, trace.SpanKindClient)
	if len(ended) != 4 || len(servers) != 2 || len(clients) != 2 {
		t.Fatalf("got spans %v, want two server and two client spans", ended)
	}
	for i, req := range got {
		assertForwardedHeaders(t, req.header)
		server, client := servers[i], clients[i]
		if server.Parent.SpanID().String() != testParentID || server.SpanContext.TraceID().String() != testTraceID {
			t.Errorf("server span %s does not continue the incoming trace", server.SpanContext.SpanID())
		}
		if client.Parent.SpanID() != server.SpanContext.SpanID() {
			t.Errorf("client span is not a child of the server span")
		}
		if client.Name != "POST" {
			t.Errorf("client span name %q, want %q", client.Name, "POST")
		}
		want := "00-" + testTraceID + "-" + client.SpanContext.SpanID().String() + "-01"
		if tp := req.header.Get("traceparent"); tp != want {
			t.Errorf("backend traceparent %q, want %q", tp, want)
		}
	}
}

func TestEchoPropagationWithoutTracing(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T)
	}{
		{name: "noop tracer provider", setup: func(t *testing.T) {}},
		{name: "SDK disabled", setup: func(t *testing.T) {
			t.Setenv("OTEL_SDK_DISABLED", "true")
			otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
			if _, err := telemetry.Setup(context.Background(), "", zap.NewNop()); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setGlobalTracing(t, noop.NewTracerProvider())
			tt.setup(t)
			backend, requests := newRecordingBackend(t)
			srv := newInstrumentedServer("/")
			srv.config.BackendURL = []string{backend.URL}

			echoWithTraceHeaders(srv)

			got := requests()
			if len(got) != 1 {
				t.Fatalf("backend got %d requests, want 1", len(got))
			}
			assertForwardedHeaders(t, got[0].header)
			if tp := got[0].header.Get("traceparent"); tp != testTraceparent {
				t.Errorf("backend traceparent %q, want %q", tp, testTraceparent)
			}
		})
	}
}

func TestEchoBackendErrorOnServerSpan(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		url     string
		body    string
	}{
		{
			name:    "call failed",
			backend: "http://podinfo:s3cret@backend.test:9898/echo?token=t0ken#frag",
			url:     "http://backend.test:9898/echo",
			body:    "call failed",
		},
		{
			name:    "invalid URL",
			backend: "http://podinfo:s3cret@backend.test:9898/echo?token=t0ken\x7f",
			body:    "call failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans := setupTracing(t)
			srv := newInstrumentedServer("/")
			srv.config.BackendURL = []string{tt.backend}
			srv.backend = newBackendClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("connection refused")
			}))

			rr := httptest.NewRecorder()
			srv.handler.ServeHTTP(rr, httptest.NewRequest("POST", "/api/echo", strings.NewReader("hello")))

			if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), tt.body) {
				t.Fatalf("got %d %q, want 200 with %q", rr.Code, rr.Body.String(), tt.body)
			}
			ended := spans.GetSpans()
			servers := spansOfKind(ended, trace.SpanKindServer)
			if len(servers) != 1 {
				t.Fatalf("got %d server spans, want 1", len(servers))
			}
			var exceptions int
			for _, e := range servers[0].Events {
				if e.Name != "exception" {
					continue
				}
				exceptions++
				if u, _ := attr(e.Attributes, "url.full"); u.AsString() != tt.url {
					t.Errorf("url.full %q, want %q", u.AsString(), tt.url)
				}
			}
			if exceptions != 1 {
				t.Errorf("got %d exception events on the server span, want 1", exceptions)
			}
			for _, c := range spansOfKind(ended, trace.SpanKindClient) {
				if u, _ := attr(c.Attributes, "url.full"); u.AsString() != tt.url {
					t.Errorf("client span url.full %q, want %q", u.AsString(), tt.url)
				}
			}
			for _, s := range ended {
				var values []string
				for _, a := range s.Attributes {
					values = append(values, a.Value.Emit())
				}
				for _, e := range s.Events {
					values = append(values, e.Name)
					for _, a := range e.Attributes {
						values = append(values, a.Value.Emit())
					}
				}
				values = append(values, s.Status.Description)
				for _, v := range values {
					if strings.Contains(v, "s3cret") || strings.Contains(v, "t0ken") {
						t.Errorf("span %q records %q", s.Name, v)
					}
				}
			}
		})
	}
}
