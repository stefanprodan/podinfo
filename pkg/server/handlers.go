package server

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
	"github.com/stefanprodan/k8s-podinfo/pkg/version"
	"gopkg.in/yaml.v2"
)

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	resp, err := makeResponse()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	d, err := yaml.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "text/x-yaml; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	w.Write(d)
}

func (s *Server) echo(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			glog.Errorf("Reading the request body failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		sha1 := hash(string(body))
		glog.Infof("Payload received from %s hash %s", r.RemoteAddr, sha1)
		w.WriteHeader(http.StatusAccepted)
		w.Write(body)
	default:
		w.WriteHeader(http.StatusNotAcceptable)
	}
}

func (s *Server) backend(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			glog.Errorf("Reading the request body failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		backendURL := os.Getenv("backend_url")
		if len(backendURL) > 0 {
			resp, err := http.Post(backendURL, r.Header.Get("Content-type"), bytes.NewReader(body))
			if err != nil {
				glog.Errorf("Backend call failed: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
			defer resp.Body.Close()
			rbody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				glog.Errorf("Reading the backend request body failed: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
			glog.Infof("Payload received from backend: %s", string(rbody))
			w.WriteHeader(http.StatusAccepted)
			w.Write(rbody)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Backend not specified, set backend_url env var"))
		}
	default:
		w.WriteHeader(http.StatusNotAcceptable)
	}
}

func (s *Server) job(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			glog.Errorf("Reading the request body failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		glog.Infof("Payload received from %s: %s", r.RemoteAddr, string(body))

		job := struct {
			Wait int `json:"wait"`
		}{
			Wait: 0,
		}
		err = json.Unmarshal(body, &job)
		if err != nil {
			glog.Errorf("Reading the request body failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		if job.Wait > 0 {
			time.Sleep(time.Duration(job.Wait) * time.Second)
		}
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Job done"))
	default:
		w.WriteHeader(http.StatusNotAcceptable)
	}
}

func (s *Server) write(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			glog.Errorf("Reading the request body failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		sha1 := hash(string(body))
		err = ioutil.WriteFile(path.Join(dataPath, sha1), body, 0644)
		if err != nil {
			glog.Errorf("Writing file to /data failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		glog.Infof("Write command received from %s hash %s", r.RemoteAddr, sha1)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(sha1))
	default:
		w.WriteHeader(http.StatusNotAcceptable)
	}
}

func (s *Server) read(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			glog.Errorf("Reading the request body failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		sha1 := string(body)
		content, err := ioutil.ReadFile(path.Join(dataPath, sha1))
		if err != nil {
			glog.Errorf("Reading file from /data/%s failed: %v", sha1, err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		glog.Infof("Read command received from %s hash %s", r.RemoteAddr, sha1)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(content))
	default:
		w.WriteHeader(http.StatusNotAcceptable)
	}
}

func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/version" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	resp := map[string]string{
		"version": version.VERSION,
		"commit":  version.GITCOMMIT,
	}

	d, err := yaml.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "text/x-yaml; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	w.Write(d)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&healthy) == 1 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&ready) == 1 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) enable(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt32(&ready, 1)
}

func (s *Server) disable(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt32(&ready, 0)
}

func (s *Server) panic(w http.ResponseWriter, r *http.Request) {
	glog.Fatal("Kill switch triggered")
}

func hash(input string) string {
	h := sha1.New()
	return fmt.Sprintf("%x", h.Sum([]byte(input)))
}
