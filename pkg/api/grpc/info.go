package grpc

import (
	"context"
	"log"
	"runtime"
	"strconv"

	"github.com/stefanprodan/podinfo/pkg/api/grpc/info"
	"github.com/stefanprodan/podinfo/pkg/version"
)

type infoServer struct {
	info.UnimplementedInfoServiceServer
	config *Config
}


func (s *infoServer) Info (ctx context.Context, message *info.InfoRequest) (*info.InfoResponse, error){

	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic:", r)
		}
	}()

	data := RuntimeResponse{
		Hostname:     s.config.Hostname,
		Version:      version.VERSION,
		Revision:     version.REVISION,
		Color:        s.config.UIColor,
		Logo:         s.config.UILogo,
		Message:      s.config.UIMessage,
		Goos:         runtime.GOOS,
		Goarch:       runtime.GOARCH,
		Runtime:      runtime.Version(),
		Numgoroutine: strconv.FormatInt(int64(runtime.NumGoroutine()), 10),
		Numcpu:       strconv.FormatInt(int64(runtime.NumCPU()), 10),
	}


	return &info.InfoResponse{
		Hostname:     data.Hostname,
		Version:      data.Version,
		Revision:     data.Revision,
		Color:        data.Color,
		Logo:         data.Logo,
		Message:      data.Message,
		Goos: 		  data.Goos,
		Goarch: 	  data.Goarch,
		Runtime: 	  data.Runtime,
		Numgoroutine: data.Numgoroutine,
		Numcpu: 	  data.Numcpu,
	}, nil

}

type RuntimeResponse struct {
	Hostname     string `json:"hostname"`
	Version      string `json:"version"`
	Revision     string `json:"revision"`
	Color        string `json:"color"`
	Logo         string `json:"logo"`
	Message      string `json:"message"`
	Goos         string `json:"goos"`
	Goarch       string `json:"goarch"`
	Runtime      string `json:"runtime"`
	Numgoroutine string `json:"num_goroutine"`
	Numcpu       string `json:"num_cpu"`
}
