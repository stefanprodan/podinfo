package server

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Response struct {
	Runtime     map[string]string `json:"runtime"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Environment map[string]string `json:"environment"`
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
	files := make([]string, 0)
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		files = append(files, path)

		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "Reading from %v failed", dir)
	}
	list := make(map[string]string, 0)
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
	runtime := map[string]string{
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"version":       runtime.Version(),
		"max_procs":     strconv.FormatInt(int64(runtime.GOMAXPROCS(0)), 10),
		"num_goroutine": strconv.FormatInt(int64(runtime.NumGoroutine()), 10),
		"num_cpu":       strconv.FormatInt(int64(runtime.NumCPU()), 10),
	}
	runtime["hostname"], _ = os.Hostname()
	return runtime
}
