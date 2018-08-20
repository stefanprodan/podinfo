package api

import (
	"net/http"

	"github.com/stefanprodan/k8s-podinfo/pkg/version"
)

func (s *Server) versionHandler(w http.ResponseWriter, r *http.Request) {
	result := map[string]string{
		"version": version.VERSION,
		"commit":  version.REVISION,
	}
	s.JSONResponse(w, r, result)
}
