package templates

import (
	promv1 "monitoring.coreos.com/servicemonitor/v1"
	timoniv1 "timoni.sh/core/v1alpha1"
)

#ServiceMonitor: promv1.#ServiceMonitor & {
	_config:  #Config
	metadata: _config.metadata
	if _config.serviceMonitor.additionalLabels != _|_ {
		metadata: labels: _config.serviceMonitor.additionalLabels
	}
	if _config.serviceMonitor.annotations != _|_ {
		metadata: annotations: _config.serviceMonitor.annotations
	}
	spec: timoniv1.#MonitorSpec & {#Values: _config.serviceMonitor}
	spec: {
		namespaceSelector: matchNames: [_config.metadata.namespace]
		selector: matchLabels: _config.selector.labels
		endpoints: [
			timoniv1.#MonitorEndpointSpec & {
				#Values: _config.serviceMonitor
				port:    "http-metrics"
				path:    "/metrics"
			},
		]
	}
}
