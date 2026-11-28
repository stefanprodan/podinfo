package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/stefanprodan/podinfo/pkg/api/limits"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
	"github.com/stefanprodan/podinfo/pkg/version"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Echo godoc
// @Summary Echo
// @Description echos the posted content, or when backend URLs are set posts it to each backend and returns their responses
// @Tags HTTP API
// @Accept json,plain,octet-stream
// @Produce octet-stream,json
// @Param payload body string false "content to echo"
// @Router /echo [post]
// @Router /api/echo [post]
// @Success 202 {file} file "the posted content, without backends"
// @Success 200 {array} string "the response or the error of each backend"
// @Failure 400 {object} http.ErrorMessage "invalid request body"
// @Failure 413 {object} http.ErrorMessage "request body too large"
// @Failure 499 {object} http.ErrorMessage "client closed request"
func (s *Server) echoHandler(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readLimitedBody(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()

	if len(s.config.BackendURL) > 0 {
		result := make([]string, len(s.config.BackendURL))
		span := trace.SpanFromContext(r.Context())
		var wg sync.WaitGroup
		wg.Add(len(s.config.BackendURL))
		for i, b := range s.config.BackendURL {
			go func(index int, backend string) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(r.Context(), s.config.HttpClientTimeout)
				defer cancel()
				display, ok := backendURL(backend)
				var attrs []attribute.KeyValue
				if ok {
					attrs = append(attrs, semconv.URLFull(display))
				}
				urlAttr := trace.WithAttributes(attrs...)

				backendReq, err := http.NewRequestWithContext(ctx, "POST", backend, bytes.NewReader(body))
				if err != nil {
					err = redactURLError(err, display)
					s.logger.Error("backend call failed", zap.Error(err), zap.String("url", display), telemetry.Context(r.Context()))
					span.RecordError(errBackendCall, urlAttr)
					result[index] = fmt.Sprintf("backend %v call failed %v", display, err)
					return
				}

				if id := r.Header.Get("x-request-id"); id != "" {
					backendReq.Header.Set("x-request-id", id)
				}
				backendReq.Header.Set("X-API-Version", version.VERSION)
				backendReq.Header.Set("X-API-Revision", version.REVISION)

				client, err := s.backendClient(backend)
				if err != nil {
					err = redactURLError(err, display)
					s.logger.Error("backend call failed", zap.Error(err), zap.String("url", display), telemetry.Context(r.Context()))
					span.RecordError(errBackendCall, urlAttr)
					result[index] = fmt.Sprintf("backend %v call failed %v", display, err)
					return
				}

				resp, err := client.Do(backendReq)
				if err != nil {
					err = redactURLError(err, display)
					if r.Context().Err() == nil {
						s.logger.Error("backend call failed", zap.Error(err), zap.String("url", display), telemetry.Context(r.Context()))
						span.RecordError(errBackendCall, urlAttr)
					}
					result[index] = fmt.Sprintf("backend %v call failed %v", display, err)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode >= 400 {
					s.logger.Error("backend call failed", zap.Int("status", resp.StatusCode), zap.String("url", display), telemetry.Context(r.Context()))
					span.RecordError(fmt.Errorf("backend response status code %v", resp.StatusCode), urlAttr)
					result[index] = fmt.Sprintf("backend %v response status code %v", display, resp.StatusCode)
					return
				}

				rbody, err := io.ReadAll(io.LimitReader(resp.Body, limits.MaxBackendResponseSize+1))
				if err == nil && len(rbody) > limits.MaxBackendResponseSize {
					err = errBackendTooLarge
				}
				if err != nil {
					if r.Context().Err() == nil {
						s.logger.Error(
							"reading the backend response body failed",
							zap.Error(err),
							zap.String("url", display),
							telemetry.Context(r.Context()))
						span.RecordError(errBackendRead, urlAttr)
					}
					result[index] = fmt.Sprintf("backend %v call failed %v", display, err)
					return
				}

				if ce := s.logger.Check(zap.DebugLevel, "payload received from backend"); ce != nil {
					fields := []zap.Field{zap.Int("size", len(rbody)), zap.String("url", display), telemetry.Context(r.Context())}
					if len(rbody) <= maxLoggedPayloadSize {
						fields = append(fields, zap.String("response", string(rbody)))
					}
					ce.Write(fields...)
				}

				result[index] = string(rbody)
			}(i, b)
		}
		wg.Wait()
		if r.Context().Err() != nil {
			s.ErrorResponse(w, r, "client closed request", statusClientClosedRequest)
			return
		}

		w.Header().Set("X-Color", s.config.UIColor)
		s.JSONResponse(w, r, result)

	} else {
		s.logPayload(r.Context(), body)
		setRawResponseHeaders(w)
		w.Header().Set("X-Color", s.config.UIColor)
		w.WriteHeader(http.StatusAccepted)
		w.Write(body)
	}
}

var (
	errBackendCall     = errors.New("backend call failed")
	errBackendRead     = errors.New("reading the backend response failed")
	errBackendTooLarge = errors.New("response body too large")
	errInvalidURL      = errors.New("invalid URL")
)

// backendURL returns the URL without userinfo, query and fragment, the form
// shown in logs, responses and traces.
func backendURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "[invalid]", false
	}
	return displayURL(u)
}

func displayURL(u *url.URL) (string, bool) {
	if u.Opaque != "" || u.Host == "" {
		return "[invalid]", false
	}
	d := *u
	d.User, d.RawQuery, d.ForceQuery, d.Fragment, d.RawFragment = nil, "", false, "", ""
	return d.String(), true
}

// redactURLError replaces the URL that net/url and net/http errors embed with
// the display URL. Parse errors can quote part of the URL, so they become
// errInvalidURL.
func redactURLError(err error, display string) error {
	ue, ok := errors.AsType[*url.Error](err)
	if !ok {
		return err
	}
	if ue.Op == "parse" {
		return errInvalidURL
	}
	return &url.Error{Op: ue.Op, URL: display, Err: ue.Err}
}

// maxLoggedPayloadSize is the largest payload logged at debug level.
const maxLoggedPayloadSize = limits.MaxLoggedPayloadSize

// logPayload logs the request body at debug level. JSON bodies are embedded
// as JSON objects in the log line, other bodies as strings. Bodies over
// maxLoggedPayloadSize are logged with their size only.
func (s *Server) logPayload(ctx context.Context, body []byte) {
	if ce := s.logger.Check(zap.DebugLevel, "payload received"); ce != nil {
		fields := []zap.Field{zap.Int("size", len(body)), telemetry.Context(ctx)}
		switch {
		case len(body) > maxLoggedPayloadSize:
		case json.Valid(body):
			fields = append(fields, zap.Reflect("payload", json.RawMessage(body)))
		default:
			fields = append(fields, zap.String("payload", string(body)))
		}
		ce.Write(fields...)
	}
}
