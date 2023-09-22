package templates

import (
	promv1 "monitoring.coreos.com/servicemonitor/v1"
)

#ServiceMonitor: promv1.#ServiceMonitor & {
	_config: #Config
	metadata: {
		name:      _config.metadata.name
		namespace: _config.metadata.namespace
		labels:    _config.metadata.labels
		if _config.metadata.annotations != _|_ {
			annotations: _config.metadata.annotations
		}
	}
	spec: {
		endpoints: [{
			path:     "/metrics"
			port:     "http-metrics"
			interval: _config.monitoring.interval
		}]
		namespaceSelector: matchNames: [_config.metadata.namespace]
		selector: matchLabels: _config.metadata.labelSelector
	}
}
