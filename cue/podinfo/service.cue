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
}

#Service: corev1.#Service & {
	_config:    #Config
	apiVersion: "v1"
	kind:       "Service"
	metadata:   _config.meta
	spec:       corev1.#ServiceSpec & {
		type:     _config.service.type
		selector: _config.selectorLabels
		ports: [
			{
				name:       "http"
				port:       _config.service.externalPort
				targetPort: "\(name)"
				protocol:   "TCP"
			},
			{
				name:       "http-metrics"
				port:       _config.service.metricsPort
				targetPort: "\(name)"
				protocol:   "TCP"
			},
			{
				name:       "grpc"
				port:       _config.service.grpcPort
				targetPort: "\(name)"
				protocol:   "TCP"
			},
		]
	}
}
