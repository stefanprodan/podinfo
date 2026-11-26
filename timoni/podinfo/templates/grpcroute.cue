package templates

import (
	gatewayv1 "gateway.networking.k8s.io/grpcroute/v1"
)

// The Gateway API GRPCRoute directing traffic from the referenced
// Gateways to the podinfo grpc Service port.
#GRPCRoute: gatewayv1.#GRPCRoute & {
	_config: #Config
	metadata: {
		name:      _config.metadata.name
		namespace: _config.metadata.namespace
		labels:    _config.metadata.labels
		if _config.grpcRoute.labels != _|_ {
			labels: _config.grpcRoute.labels
		}
		if _config.metadata.annotations != _|_ {
			annotations: _config.metadata.annotations
		}
		if _config.grpcRoute.annotations != _|_ {
			annotations: _config.grpcRoute.annotations
		}
	}
	spec: {
		// The parent group and kind default to Gateway API Gateways,
		// rendered explicitly so that the CEL rules validating the
		// parent references evaluate without the CRD defaulting.
		if _config.grpcRoute.parentRefs != _|_ {
			parentRefs: [for p in _config.grpcRoute.parentRefs {
				group: *"gateway.networking.k8s.io" | string
				kind:  *"Gateway" | string
				p
			}]
		}
		if _config.grpcRoute.hostnames != _|_ {
			hostnames: _config.grpcRoute.hostnames
		}
		rules: [for r in _config.grpcRoute.rules {
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
				port:   _config.service.grpcPort
				weight: 1
			}]
		}]
	}
}
