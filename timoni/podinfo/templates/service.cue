package templates

import (
	corev1 "k8s.io/api/core/v1"
)

#Service: corev1.#Service & {
	_config:    #Config
	apiVersion: "v1"
	kind:       "Service"
	metadata:   _config.metadata
	metadata: {
		if _config.service.labels != _|_ {
			labels: _config.service.labels
		}
		if _config.service.annotations != _|_ {
			annotations: _config.service.annotations
		}
	}
	spec: corev1.#ServiceSpec & {
		type:     corev1.#ServiceTypeClusterIP
		selector: _config.selector.labels
		ports: [
			{
				name:       "http"
				port:       _config.service.port
				targetPort: "\(name)"
				protocol:   "TCP"
			},
			if _config.monitoring.enabled {
				{
					name:       "http-metrics"
					port:       9797
					targetPort: "http-metrics"
					protocol:   "TCP"
				}
			},
		]
	}
}
