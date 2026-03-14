package http

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"

	"github.com/gorilla/mux"
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
func (s *Server) storeWriteHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "storeWriteHandler")
	defer span.End()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.ErrorResponse(w, r, span, "reading the request body failed", http.StatusBadRequest)
		return
	}

	hash := hash(string(body))
	err = os.WriteFile(path.Join(s.config.DataPath, hash), body, 0644)
	if err != nil {
		s.logger.Warn("writing file failed", zap.Error(err), zap.String("file", path.Join(s.config.DataPath, hash)))
		s.ErrorResponse(w, r, span, "writing file failed", http.StatusInternalServerError)
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
func (s *Server) storeReadHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "storeReadHandler")
	defer span.End()

	hash := mux.Vars(r)["hash"]
	if !validHash.MatchString(hash) {
		s.ErrorResponse(w, r, span, "invalid hash", http.StatusBadRequest)
		return
	}
	content, err := os.ReadFile(path.Join(s.config.DataPath, hash))
	if err != nil {
		s.logger.Warn("reading file failed", zap.Error(err), zap.String("file", path.Join(s.config.DataPath, hash)))
		s.ErrorResponse(w, r, span, "reading file failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(content))
}

func hash(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}
