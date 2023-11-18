package templates

import (
	promv1 "monitoring.coreos.com/servicemonitor/v1"
)

#ServiceMonitor: promv1.#ServiceMonitor & {
	_config:  #Config
	metadata: _config.metadata
	spec: {
		endpoints: [{
			path:     "/metrics"
			port:     "http-metrics"
			interval: "\(_config.monitoring.interval)s"
		}]
		namespaceSelector: matchNames: [_config.metadata.namespace]
		selector: matchLabels: _config.selector.labels
	}
}
