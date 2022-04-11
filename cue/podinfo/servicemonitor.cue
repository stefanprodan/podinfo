package podinfo

#serviceMonConfig: {
	enabled:  *false | bool
	interval: *"15s" | string
	matchLabels: {}
}

#ServiceMonitor: {
	_config:    #Config
	apiVersion: "monitoring.coreos.com/v1"
	kind:       "ServiceMonitor"
	metadata:   _config.meta
	spec: {
		endpoints: [{
			path:     "/metrics"
			port:     "http"
			interval: _config.serviceMonitor.interval
		}]
		namespaceSelector: matchNames: _config.meta.namespace
		selector: matchLabels:         _config.selectorLabels
	}
}
