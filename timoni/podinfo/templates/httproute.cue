package templates

import (
	gatewayv1 "gateway.networking.k8s.io/httproute/v1"
)

// The Gateway API HTTPRoute directing traffic from the referenced
// Gateways to the podinfo http Service port.
#HTTPRoute: gatewayv1.#HTTPRoute & {
	_config: #Config
	metadata: {
		name:      _config.metadata.name
		namespace: _config.metadata.namespace
		labels:    _config.metadata.labels
		if _config.httpRoute.labels != _|_ {
			labels: _config.httpRoute.labels
		}
		if _config.metadata.annotations != _|_ {
			annotations: _config.metadata.annotations
		}
		if _config.httpRoute.annotations != _|_ {
			annotations: _config.httpRoute.annotations
		}
	}
	spec: {
		// The parent group and kind default to Gateway API Gateways,
		// rendered explicitly so that the CEL rules validating the
		// parent references evaluate without the CRD defaulting.
		if _config.httpRoute.parentRefs != _|_ {
			parentRefs: [for p in _config.httpRoute.parentRefs {
				group: *"gateway.networking.k8s.io" | string
				kind:  *"Gateway" | string
				p
			}]
		}
		if _config.httpRoute.hostnames != _|_ {
			hostnames: _config.httpRoute.hostnames
		}
		rules: [for r in _config.httpRoute.rules {
			if r.matches != _|_ {
				matches: r.matches
			}
			if r.filters != _|_ {
				filters: r.filters
			}
			backendRefs: [{
				group:  ""
				kind:   "Service"
				name:   _config.metadata.name
				port:   _config.service.port
				weight: 1
			}]
		}]
	}
}
