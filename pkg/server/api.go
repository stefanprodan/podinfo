package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/stefanprodan/k8s-podinfo/pkg/version"
)

func (s *Server) apiInfo(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/info" && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	host, _ := os.Hostname()
	color := os.Getenv("color")
	if len(color) < 1 {
		color = "blue"
	}

	msg := os.Getenv("message")
	if len(msg) < 1 {
		msg = fmt.Sprintf("Greetings from podinfo v%v", version.VERSION)
	}

	data := struct {
		Message  string `json:"message"`
		Version  string `json:"version"`
		Revision string `json:"revision"`
		Hostname string `json:"hostname"`
		Color    string `json:"color"`
	}{
		Message:  msg,
		Version:  version.VERSION,
		Revision: version.GITCOMMIT,
		Hostname: host,
		Color:    color,
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
