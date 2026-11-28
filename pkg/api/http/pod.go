package http

import (
	"net/http"
	"runtime"
	"strconv"

	"github.com/stefanprodan/podinfo/pkg/environ"
	"github.com/stefanprodan/podinfo/pkg/version"
)

// Pod godoc
// @Summary Pod runtime, environment and request headers
// @Description returns the runtime information, the environment variables and the request HTTP headers of the pod that served the request, with secret values masked, plus X-Spiffe-Id when the client presented a SPIFFE X.509 SVID
// @Tags HTTP API
// @Produce json
// @Success 200 {object} http.PodResponse
// @Router /api/pod [get]
func (s *Server) podHandler(w http.ResponseWriter, r *http.Request) {
	s.JSONResponse(w, r, PodResponse{
		Hostname:     s.config.Hostname,
		Version:      version.VERSION,
		Revision:     version.REVISION,
		GOOS:         runtime.GOOS,
		GOARCH:       runtime.GOARCH,
		Runtime:      runtime.Version(),
		NumGoroutine: strconv.Itoa(runtime.NumGoroutine()),
		NumCPU:       strconv.Itoa(runtime.NumCPU()),
		Env:          environ.RedactedMap(),
		Headers:      echoedHeaders(r),
	})
}

// PodResponse is the body of /api/pod: the runtime information, the
// environment and the request headers, with secrets masked.
type PodResponse struct {
	Hostname     string              `json:"hostname"`
	Version      string              `json:"version"`
	Revision     string              `json:"revision"`
	GOOS         string              `json:"goos"`
	GOARCH       string              `json:"goarch"`
	Runtime      string              `json:"runtime"`
	NumGoroutine string              `json:"num_goroutine"`
	NumCPU       string              `json:"num_cpu"`
	Env          map[string]string   `json:"env"`
	Headers      map[string][]string `json:"headers"`
}
