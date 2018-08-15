package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"
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

func (s *Server) apiEcho(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/echo" && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error().Msgf("Reading the request body failed: %v", err)
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	backendURL := os.Getenv("backendURL")
	if len(backendURL) > 0 {
		backendReq, err := http.NewRequest("POST", backendURL, bytes.NewReader(body))
		if err != nil {
			log.Error().Err(err).Msgf("%v backend call failed", r.URL.Path)
			jsonError(w, "backend call failed", http.StatusInternalServerError)
			return
		}

		// forward headers
		copyTracingHeaders(r, backendReq)
		setVersionHeaders(backendReq)

		// TODO: make the timeout configurable
		ctx, cancel := context.WithTimeout(backendReq.Context(), 2*time.Minute)
		defer cancel()

		// call backend
		resp, err := http.DefaultClient.Do(backendReq.WithContext(ctx))
		if err != nil {
			log.Error().Msgf("%v backend call failed", r.URL.Path)
			jsonError(w, "backend call failed", http.StatusInternalServerError)
			return
		}

		defer resp.Body.Close()

		// copy error status from backend and exit
		if resp.StatusCode >= 400 {
			w.WriteHeader(resp.StatusCode)
			return
		}

		// forward the received body
		rbody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error().Err(err).Msgf("%v reading the backend request body failed", r.URL.Path)
			jsonError(w, "backend call failed", http.StatusInternalServerError)
			return
		}

		// set logLevel=info when load testing
		log.Debug().Msgf("Payload received %v from backend: %s", r.URL.Path, string(rbody))

		setResponseHeaders(w)
		w.Write(rbody)
	} else {
		setResponseHeaders(w)
		w.Write(body)
	}
}

func copyTracingHeaders(from *http.Request, to *http.Request) {
	headers := []string{
		"x-request-id",
		"x-b3-traceid",
		"x-b3-spanid",
		"x-b3-parentspanid",
		"x-b3-sampled",
		"x-b3-flags",
		"x-ot-span-context",
	}

	for i := range headers {
		headerValue := from.Header.Get(headers[i])
		if len(headerValue) > 0 {
			to.Header.Set(headers[i], headerValue)
		}
	}
}

func setVersionHeaders(r *http.Request) {
	r.Header.Set("X-API-Version", version.VERSION)
	r.Header.Set("X-API-Revision", version.GITCOMMIT)
}

func setResponseHeaders(w http.ResponseWriter) {
	color := os.Getenv("color")
	if len(color) < 1 {
		color = "blue"
	}
	w.Header().Set("X-Color", color)
	w.WriteHeader(http.StatusAccepted)
}

func jsonError(w http.ResponseWriter, error string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)

	data := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}{
		Code:    code,
		Message: error,
	}

	body, err := json.Marshal(data)
	if err != nil {
		log.Debug().Err(err).Msg("jsonError marshal failed")
	} else {
		w.Write(body)
	}
}
