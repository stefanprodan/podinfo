package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/stefanprodan/k8s-podinfo/pkg/version"
)

func (s *Server) apiInfo(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/info" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	host, _ := os.Hostname()
	data := struct {
		Title    string `json:"title"`
		Message  string `json:"message"`
		Version  string `json:"version"`
		Revision string `json:"revision"`
		Hostname string `json:"hostname"`
		Color    string `json:"color"`
	}{
		Title:    fmt.Sprintf("podinfo v%v", version.VERSION),
		Message:  fmt.Sprintf("Hello from podinfo v%v Git commit %v", version.VERSION, version.GITCOMMIT),
		Version:  version.VERSION,
		Revision: version.GITCOMMIT,
		Hostname: host,
		Color:    "green",
	}

	d, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	w.Write(d)
}
