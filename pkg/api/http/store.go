package http

import (
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"os"
	"path"
	"regexp"

	"github.com/gorilla/mux"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var validHash = regexp.MustCompile(`^[a-f0-9]{40}$`)

// Store godoc
// @Summary Upload file
// @Description writes the posted content to disk at /data/hash and returns the SHA1 hash of the content
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /store [post]
// @Success 200 {object} http.MapResponse
// @Failure 400 {object} http.ErrorMessage "invalid request body"
// @Failure 413 {object} http.ErrorMessage "request body too large"
// @Failure 500 {object} http.ErrorMessage "writing file failed"
func (s *Server) storeWriteHandler(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readLimitedBody(w, r)
	if !ok {
		return
	}

	hash := hash(string(body))
	file := path.Join(s.config.DataPath, hash)
	_, span := s.tracer.Start(r.Context(), "store write", trace.WithAttributes(semconv.FilePath(file)))
	err := os.WriteFile(file, body, 0644)
	endSpan(span, err)
	if err != nil {
		s.logger.Warn("writing file failed", zap.Error(err), zap.String("file", file))
		s.ErrorResponse(w, r, "writing file failed", http.StatusInternalServerError)
		return
	}
	s.JSONResponseCode(w, r, map[string]string{"hash": hash}, http.StatusAccepted)
}

// Store godoc
// @Summary Download file
// @Description returns the content of the file /data/hash if exists
// @Tags HTTP API
// @Accept json
// @Produce plain
// @Param hash path string true "hash value"
// @Router /store/{hash} [get]
// @Success 200 {string} string "file"
// @Failure 400 {object} http.ErrorMessage "invalid hash"
// @Failure 500 {object} http.ErrorMessage "reading file failed"
func (s *Server) storeReadHandler(w http.ResponseWriter, r *http.Request) {
	hash := mux.Vars(r)["hash"]
	if !validHash.MatchString(hash) {
		s.ErrorResponse(w, r, "invalid hash", http.StatusBadRequest)
		return
	}
	file := path.Join(s.config.DataPath, hash)
	_, span := s.tracer.Start(r.Context(), "store read", trace.WithAttributes(semconv.FilePath(file)))
	content, err := os.ReadFile(file)
	endSpan(span, err)
	if err != nil {
		s.logger.Warn("reading file failed", zap.Error(err), zap.String("file", file))
		s.ErrorResponse(w, r, "reading file failed", http.StatusInternalServerError)
		return
	}
	setRawResponseHeaders(w)
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(content))
}

func hash(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}
