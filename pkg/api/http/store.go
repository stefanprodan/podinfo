package http

import (
	"errors"
	"io/fs"
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/store"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

// Store godoc
// @Summary Upload file
// @Description writes the posted content to a file named by its SHA1 hash in the data path and returns the hash
// @Tags HTTP API
// @Accept json,plain,octet-stream
// @Produce json
// @Param payload body string false "content to store"
// @Router /store [post]
// @Router /store [put]
// @Success 202 {object} http.MapResponse
// @Failure 400 {object} http.ErrorMessage "invalid request body"
// @Failure 413 {object} http.ErrorMessage "request body too large"
// @Failure 500 {object} http.ErrorMessage "writing file failed"
func (s *Server) storeWriteHandler(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readLimitedBody(w, r)
	if !ok {
		return
	}

	hash, err := s.store.Write(r.Context(), body)
	if err != nil {
		s.logger.Warn("writing file failed", zap.Error(err), telemetry.Context(r.Context()))
		s.ErrorResponse(w, r, "writing file failed", http.StatusInternalServerError)
		return
	}
	s.JSONResponseCode(w, r, map[string]string{"hash": hash}, http.StatusAccepted)
}

// Store godoc
// @Summary Download file
// @Description returns the content of the file named by the hash in the data path
// @Tags HTTP API
// @Produce octet-stream,json
// @Param hash path string true "SHA1 hash of the content"
// @Router /store/{hash} [get]
// @Success 200 {file} file "file content"
// @Failure 400 {object} http.ErrorMessage "invalid hash"
// @Failure 404 {object} http.ErrorMessage "file not found"
// @Failure 500 {object} http.ErrorMessage "reading file failed"
func (s *Server) storeReadHandler(w http.ResponseWriter, r *http.Request) {
	content, err := s.store.Read(r.Context(), mux.Vars(r)["hash"])
	if errors.Is(err, store.ErrInvalidHash) {
		s.ErrorResponse(w, r, "invalid hash", http.StatusBadRequest)
		return
	}
	if err != nil {
		s.logger.Warn("reading file failed", zap.Error(err), telemetry.Context(r.Context()))
		if errors.Is(err, fs.ErrNotExist) {
			s.ErrorResponse(w, r, "file not found", http.StatusNotFound)
			return
		}
		s.ErrorResponse(w, r, "reading file failed", http.StatusInternalServerError)
		return
	}
	setRawResponseHeaders(w)
	_, _ = w.Write(content) //nolint:gosec // the raw response headers stop browsers from rendering it
}
