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

	"github.com/stefanprodan/podinfo/pkg/telemetry"
	"github.com/stefanprodan/podinfo/pkg/version"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Echo godoc
// @Summary Echo
// @Description forwards the call to the backend service and echos the posted content
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /api/echo [post]
// @Success 202 {object} http.MapResponse
// @Failure 400 {object} http.ErrorMessage "invalid request body"
// @Failure 413 {object} http.ErrorMessage "request body too large"
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
				backendURL := trace.WithAttributes(redactedURL(backend)...)

				backendReq, err := http.NewRequestWithContext(ctx, "POST", backend, bytes.NewReader(body))
				if err != nil {
					s.logger.Error("backend call failed", zap.Error(err), zap.String("url", backend), telemetry.Context(r.Context()))
					span.RecordError(errBackendCall, backendURL)
					result[index] = fmt.Sprintf("backend %v call failed %v", backend, err)
					return
				}

				if id := r.Header.Get("x-request-id"); id != "" {
					backendReq.Header.Set("x-request-id", id)
				}
				backendReq.Header.Set("X-API-Version", version.VERSION)
				backendReq.Header.Set("X-API-Revision", version.REVISION)

				client, err := s.backendClient(backend)
				if err != nil {
					s.logger.Error("backend call failed", zap.Error(err), zap.String("url", backend), telemetry.Context(r.Context()))
					span.RecordError(errBackendCall, backendURL)
					result[index] = fmt.Sprintf("backend %v call failed %v", backend, err)
					return
				}

				// call backend
				resp, err := client.Do(backendReq)
				if err != nil {
					s.logger.Error("backend call failed", zap.Error(err), zap.String("url", backend), telemetry.Context(r.Context()))
					span.RecordError(errBackendCall, backendURL)
					result[index] = fmt.Sprintf("backend %v call failed %v", backend, err)
					return
				}
				defer resp.Body.Close()

				// copy error status from backend and exit
				if resp.StatusCode >= 400 {
					s.logger.Error("backend call failed", zap.Int("status", resp.StatusCode), zap.String("url", backend), telemetry.Context(r.Context()))
					span.RecordError(fmt.Errorf("backend response status code %v", resp.StatusCode), backendURL)
					result[index] = fmt.Sprintf("backend %v response status code %v", backend, resp.StatusCode)
					return
				}

				// forward the received body
				rbody, err := io.ReadAll(resp.Body)
				if err != nil {
					s.logger.Error(
						"reading the backend request body failed",
						zap.Error(err),
						zap.String("url", backend),
						telemetry.Context(r.Context()))
					span.RecordError(errBackendRead, backendURL)
					result[index] = fmt.Sprintf("backend %v call failed %v", backend, err)
					return
				}

				s.logger.Debug(
					"payload received from backend",
					zap.String("response", string(rbody)),
					zap.String("url", backend),
					telemetry.Context(r.Context()))

				result[index] = string(rbody)
			}(i, b)
		}
		wg.Wait()

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
	errBackendCall = errors.New("backend call failed")
	errBackendRead = errors.New("reading the backend response failed")
)

func redactedURL(raw string) []attribute.KeyValue {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	u.User, u.RawQuery, u.ForceQuery, u.Fragment, u.RawFragment = nil, "", false, "", ""
	return []attribute.KeyValue{semconv.URLFull(u.String())}
}

// maxLoggedPayloadSize is the largest request body logged by logPayload.
const maxLoggedPayloadSize = 64 << 10 // 64 KiB

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
