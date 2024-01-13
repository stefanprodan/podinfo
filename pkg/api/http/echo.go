package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"sync"

	"github.com/stefanprodan/podinfo/pkg/version"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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
func (s *Server) echoHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.tracer.Start(r.Context(), "echoHandler")
	defer span.End()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("reading the request body failed", zap.Error(err))
		s.ErrorResponse(w, r, span, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	if len(s.config.BackendURL) > 0 {
		result := make([]string, len(s.config.BackendURL))
		var wg sync.WaitGroup
		wg.Add(len(s.config.BackendURL))
		for i, b := range s.config.BackendURL {
			go func(index int, backend string) {
				defer wg.Done()

				ctx = httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx))
				ctx, cancel := context.WithTimeout(ctx, s.config.HttpClientTimeout)
				defer cancel()

				backendReq, err := http.NewRequestWithContext(ctx, "POST", backend, bytes.NewReader(body))
				if err != nil {
					s.logger.Error("backend call failed", zap.Error(err), zap.String("url", backend))
					return
				}

				// forward headers
				copyTracingHeaders(r, backendReq)

				backendReq.Header.Set("X-API-Version", version.VERSION)
				backendReq.Header.Set("X-API-Revision", version.REVISION)

				// call backend
				resp, err := client.Do(backendReq)
				if err != nil {
					s.logger.Error("backend call failed", zap.Error(err), zap.String("url", backend))
					result[index] = fmt.Sprintf("backend %v call failed %v", backend, err)
					return
				}
				defer resp.Body.Close()

				// copy error status from backend and exit
				if resp.StatusCode >= 400 {
					s.logger.Error("backend call failed", zap.Int("status", resp.StatusCode), zap.String("url", backend))
					result[index] = fmt.Sprintf("backend %v response status code %v", backend, resp.StatusCode)
					return
				}

				// forward the received body
				rbody, err := io.ReadAll(resp.Body)
				if err != nil {
					s.logger.Error(
						"reading the backend request body failed",
						zap.Error(err),
						zap.String("url", backend))
					result[index] = fmt.Sprintf("backend %v call failed %v", backend, err)
					return
				}

				s.logger.Debug(
					"payload received from backend",
					zap.String("response", string(rbody)),
					zap.String("url", backend))

				result[index] = string(rbody)
			}(i, b)
		}
		wg.Wait()

		w.Header().Set("X-Color", s.config.UIColor)
		s.JSONResponse(w, r, result)

	} else {
		w.Header().Set("X-Color", s.config.UIColor)
		w.WriteHeader(http.StatusAccepted)
		w.Write(body)
	}
}

func copyTracingHeaders(from *http.Request, to *http.Request) {
	headers := []string{
		"x-request-id",
		"x-b3-traceid",
		"x-b3-spanid",
		"x-b3-parentspanid",
		"x-b3-sampled",
		"x-b3-flags",
		"x-ot-span-context",
	}

	for i := range headers {
		headerValue := from.Header.Get(headers[i])
		if len(headerValue) > 0 {
			to.Header.Set(headers[i], headerValue)
		}
	}
}
