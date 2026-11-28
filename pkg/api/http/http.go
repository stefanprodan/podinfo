// Copyright 2018 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

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

// maxRequestBodySize caps the request body the handlers buffer in memory, so
// a client cannot exhaust the memory or, through /store, the disk.
const maxRequestBodySize = limits.MaxBodySize

// RandomErrorMiddleware fails one request in three with a 500, 400 or 409.
type RandomErrorMiddleware struct {
	intn func(n int) int
}

// NewRandomErrorMiddleware returns a middleware that makes the server
// unreliable on purpose.
func NewRandomErrorMiddleware() *RandomErrorMiddleware {
	return &RandomErrorMiddleware{intn: rand.Intn}
}

// Handler returns next wrapped so that a failed request never reaches it.
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
		r.Header.Set("X-Api-Version", version.VERSION)
		r.Header.Set("X-Api-Revision", version.REVISION)

		next.ServeHTTP(w, r)
	})
}

// JSONResponse writes result as indented JSON with status 200. It answers 500
// with an empty body when result cannot be marshalled.
func (s *Server) JSONResponse(w http.ResponseWriter, r *http.Request, result any) {
	body, err := json.Marshal(result)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.Error("JSON marshal failed", zap.Error(err), telemetry.Context(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(prettyJSON(body))
}

// JSONResponseCode is JSONResponse with the given status code.
func (s *Server) JSONResponseCode(w http.ResponseWriter, r *http.Request, result any, responseCode int) {
	body, err := json.Marshal(result)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.Error("JSON marshal failed", zap.Error(err), telemetry.Context(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(responseCode)
	_, _ = w.Write(prettyJSON(body))
}

// ErrorMessage is the JSON body of an error response.
type ErrorMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse writes msg and code as an ErrorMessage with status code. A
// 5xx code is also recorded as an error on the request span.
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
	_, _ = w.Write(prettyJSON(body))
}

// readLimitedBody reads the request body up to maxRequestBodySize. It answers
// 413 for a larger body and 400 for any other read error, and returns false.
func (s *Server) readLimitedBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
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
	_ = json.Indent(&out, b, "", "  ")
	return out.Bytes()
}
