package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/stefanprodan/podinfo/pkg/version"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// maxRequestBodySize caps how much of a request body the body-reading handlers
// buffer into memory. Without this bound an unauthenticated client can POST an
// arbitrarily large body and exhaust process memory (and, for /store, disk).
const maxRequestBodySize = 10 << 20 // 10 MiB

func randomErrorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rand.Seed(time.Now().Unix())
		if rand.Int31n(3) == 0 {

			errors := []int{http.StatusInternalServerError, http.StatusBadRequest, http.StatusConflict}
			w.WriteHeader(errors[rand.Intn(len(errors))])
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
		s.logger.Error("JSON marshal failed", zap.Error(err))
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
		s.logger.Error("JSON marshal failed", zap.Error(err))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(responseCode)
	w.Write(prettyJSON(body))
}

func (s *Server) ErrorResponse(w http.ResponseWriter, r *http.Request, span trace.Span, error string, code int) {
	data := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}{
		Code:    code,
		Message: error,
	}

	span.SetStatus(codes.Error, error)

	body, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.Error("JSON marshal failed", zap.Error(err))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	w.Write(prettyJSON(body))
}

// readLimitedBody reads the request body up to maxRequestBodySize. It returns
// the body and true on success. On an oversized body it writes a 413 response,
// on any other read error a 400, and returns false so the caller returns early.
func (s *Server) readLimitedBody(w http.ResponseWriter, r *http.Request, span trace.Span) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			s.ErrorResponse(w, r, span, "request body too large", http.StatusRequestEntityTooLarge)
			return nil, false
		}
		s.logger.Error("reading the request body failed", zap.Error(err))
		s.ErrorResponse(w, r, span, "invalid request body", http.StatusBadRequest)
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
