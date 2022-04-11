package podinfo

import (
	corev1 "k8s.io/api/core/v1"
)

#serviceConfig: {
	type:         *"ClusterIP" | string
	externalPort: *9898 | int
	httpPort:     *9898 | int
	metricsPort:  *9797 | int
	grpcPort:     *9999 | int
	grpcService:  "podinfo" | string
	nodePort:     *31198 | int
}

#Service: corev1.#Service & {
	_config:    #Config
	apiVersion: "v1"
	kind:       "Service"
	metadata:   _config.meta
	spec:       corev1.#ServiceSpec & {
		type:     "ClusterIP"
		selector: #selectorLabels
		ports: [{
			name:       "http"
			port:       _config.service.externalPort
			targetPort: _config.service.httpPort
			protocol:   "TCP"
		}, if _config.tls.enabled == true {
			name:       "https"
			port:       _config.tls.port
			targetPort: "https"
			protocol:   "TCP"
		}, if _config.service.grpcPort != _|_ {
			name:       "grpc"
			port:       _config.service.grpcPort
			targetPort: "grpc"
			protocol:   "TCP"
		},
		]
	}
}
