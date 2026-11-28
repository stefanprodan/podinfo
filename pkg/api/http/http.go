package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"

	"github.com/stefanprodan/podinfo/pkg/api/limits"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
	"github.com/stefanprodan/podinfo/pkg/version"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// maxRequestBodySize caps how much of a request body the body-reading handlers
// buffer into memory. Without this bound an unauthenticated client can POST an
// arbitrarily large body and exhaust process memory (and, for /store, disk).
const maxRequestBodySize = limits.MaxBodySize

type RandomErrorMiddleware struct {
	intn func(n int) int
}

func NewRandomErrorMiddleware() *RandomErrorMiddleware {
	return &RandomErrorMiddleware{intn: rand.Intn}
}

func (m *RandomErrorMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.intn(3) == 0 {

			errors := []int{http.StatusInternalServerError, http.StatusBadRequest, http.StatusConflict}
			w.WriteHeader(errors[m.intn(len(errors))])
			return
		}
		next.ServeHTTP(w, r)
	})
}

func versionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-API-Version", version.VERSION)
		r.Header.Set("X-API-Revision", version.REVISION)

		next.ServeHTTP(w, r)
	})
}

func (s *Server) JSONResponse(w http.ResponseWriter, r *http.Request, result interface{}) {
	body, err := json.Marshal(result)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.Error("JSON marshal failed", zap.Error(err), telemetry.Context(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	w.Write(prettyJSON(body))
}

func (s *Server) JSONResponseCode(w http.ResponseWriter, r *http.Request, result interface{}, responseCode int) {
	body, err := json.Marshal(result)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.Error("JSON marshal failed", zap.Error(err), telemetry.Context(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(responseCode)
	w.Write(prettyJSON(body))
}

type ErrorMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) ErrorResponse(w http.ResponseWriter, r *http.Request, msg string, code int) {
	data := ErrorMessage{
		Code:    code,
		Message: msg,
	}

	if code >= http.StatusInternalServerError {
		trace.SpanFromContext(r.Context()).RecordError(errors.New(msg))
	}

	body, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.Error("JSON marshal failed", zap.Error(err), telemetry.Context(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	w.Write(prettyJSON(body))
}

// readLimitedBody reads the request body up to maxRequestBodySize. It returns
// the body and true on success. On an oversized body it writes a 413 response,
// on any other read error a 400, and returns false so the caller returns early.
func (s *Server) readLimitedBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			s.ErrorResponse(w, r, "request body too large", http.StatusRequestEntityTooLarge)
			return nil, false
		}
		s.logger.Error("reading the request body failed", zap.Error(err), telemetry.Context(r.Context()))
		s.ErrorResponse(w, r, "invalid request body", http.StatusBadRequest)
		return nil, false
	}
	return body, true
}

// setRawResponseHeaders prevents XSS by ensuring browsers never interpret raw responses as HTML.
func setRawResponseHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
}

func prettyJSON(b []byte) []byte {
	var out bytes.Buffer
	json.Indent(&out, b, "", "  ")
	return out.Bytes()
}
