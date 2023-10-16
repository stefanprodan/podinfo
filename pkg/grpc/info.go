package grpc

import (
	"context"
	"log"
	"runtime"
	"strconv"

	"github.com/stefanprodan/podinfo/pkg/grpc/info"
	"github.com/stefanprodan/podinfo/pkg/version"
)

type infoServer struct {
	info.UnimplementedInfoServiceServer
	config *Config
}

// type Config struct {
// 	Port        int    `mapstructure:"grpc-port"`
// 	ServiceName string `mapstructure:"grpc-service-name"`

// 	BackendURL            []string      `mapstructure:"backend-url"`
// 	UILogo                string        `mapstructure:"ui-logo"`
// 	UIMessage             string        `mapstructure:"ui-message"`
// 	UIColor               string        `mapstructure:"ui-color"`
// 	UIPath                string        `mapstructure:"ui-path"`
// 	DataPath              string        `mapstructure:"data-path"`
// 	ConfigPath            string        `mapstructure:"config-path"`
// 	CertPath              string        `mapstructure:"cert-path"`
// 	Host                  string        `mapstructure:"host"`
// 	//Port                  string        `mapstructure:"port"`
// 	SecurePort            string        `mapstructure:"secure-port"`
// 	PortMetrics           int           `mapstructure:"port-metrics"`
// 	Hostname              string        `mapstructure:"hostname"`
// 	H2C                   bool          `mapstructure:"h2c"`
// 	RandomDelay           bool          `mapstructure:"random-delay"`
// 	RandomDelayUnit       string        `mapstructure:"random-delay-unit"`
// 	RandomDelayMin        int           `mapstructure:"random-delay-min"`
// 	RandomDelayMax        int           `mapstructure:"random-delay-max"`
// 	RandomError           bool          `mapstructure:"random-error"`
// 	Unhealthy             bool          `mapstructure:"unhealthy"`
// 	Unready               bool          `mapstructure:"unready"`
// 	JWTSecret             string        `mapstructure:"jwt-secret"`
// 	CacheServer           string        `mapstructure:"cache-server"`

// }

func NewInfoServer(config *Config) (*infoServer, error) {
	infosrv := &infoServer{
		config: config,
	}

	return infosrv, nil
}

func (s *infoServer) Info (ctx context.Context, message *info.InfoRequest) (*info.InfoResponse, error){
	log.Printf("Received message body from client: hardcode")
	log.Printf("Received message body from client: %s", runtime.GOOS)

	if(s.config == nil) {log.Printf("S.config is nil")}

	return &info.InfoResponse {
		Hostname:     s.config.Hostname,
		Version:      version.VERSION,
		Revision:     version.REVISION,
		Logo:         s.config.UILogo,
		Color:        s.config.UIColor,
		Message:      s.config.UIMessage,
		Goos: 		  runtime.GOOS,
		Goarch: 	  runtime.GOARCH,
		Runtime: 	  runtime.Version(),
		Numgoroutine: strconv.FormatInt(int64(runtime.NumGoroutine()), 10),
		Numcpu: 	  strconv.FormatInt(int64(runtime.NumCPU()), 10),

	}, nil
}


