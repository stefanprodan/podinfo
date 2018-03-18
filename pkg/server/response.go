package server

import (
	"bufio"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type Response struct {
	Runtime     map[string]string `json:"runtime" yaml:"runtime"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Environment map[string]string `json:"environment" yaml:"environment"`
}

func makeResponse() (*Response, error) {
	labels, err := filesToMap("/etc/podinfod/metadata/labels")
	if err != nil {
		return nil, err
	}

	annotations, err := filesToMap("/etc/podinfod/metadata/annotations")
	if err != nil {
		return nil, err
	}

	resp := &Response{
		Environment: envToMap(),
		Runtime:     runtimeToMap(),
		Labels:      labels,
		Annotations: annotations,
	}

	return resp, nil
}

func filesToMap(dir string) (map[string]string, error) {
	list := make(map[string]string, 0)
	if _, err := os.Stat(dir); err != nil {
		// path not found
		return list, nil
	}
	files := make([]string, 0)
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		files = append(files, path)

		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "Reading from %v failed", dir)
	}
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		s := bufio.NewScanner(file)
		for s.Scan() {
			kv := strings.Split(s.Text(), "=")
			if len(kv) > 1 {
				list[kv[0]] = strings.Replace(kv[1], "\"", "", -1)
			} else {
				list[kv[0]] = ""
			}
		}
		file.Close()
	}
	return list, nil
}

func envToMap() map[string]string {
	list := make(map[string]string, 0)
	for _, env := range os.Environ() {
		kv := strings.Split(env, "=")
		if len(kv) > 1 {
			list[kv[0]] = strings.Replace(kv[1], "\"", "", -1)
		} else {
			list[kv[0]] = ""
		}
	}
	return list
}

func runtimeToMap() map[string]string {
	info := map[string]string{
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"version":       runtime.Version(),
		"max_procs":     strconv.FormatInt(int64(runtime.GOMAXPROCS(0)), 10),
		"num_goroutine": strconv.FormatInt(int64(runtime.NumGoroutine()), 10),
		"num_cpu":       strconv.FormatInt(int64(runtime.NumCPU()), 10),
		"external_ip":   findIp("http://api.ipify.org"),
	}
	return info
}

func findIp(url string) string {
	ip := ""
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: false,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: time.Duration(1 * time.Second),
	}

	req, _ := http.NewRequest("GET", url, nil)
	res, err := client.Do(req)
	if err != nil {
		log.Error().Err(errors.Wrapf(err, "cannot connect to %s", url)).Msg("ipify timeout")
		return ip
	}

	if res.Body != nil {
		defer res.Body.Close()

		if res.StatusCode == http.StatusOK {
			contents, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return ip
			}
			return string(contents)
		}
	}

	return ip
}
