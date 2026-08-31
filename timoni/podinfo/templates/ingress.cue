package templates

import (
	networkingv1 "k8s.io/api/networking/v1"
)

// The Ingress routing to the podinfo http Service port.
#Ingress: networkingv1.#Ingress & {
	_config:    #Config
	apiVersion: "networking.k8s.io/v1"
	kind:       "Ingress"
	metadata:   _config.metadata
	if _config.ingress.labels != _|_ {
		metadata: labels: _config.ingress.labels
	}
	if _config.ingress.annotations != _|_ {
		metadata: annotations: _config.ingress.annotations
	}
	spec: networkingv1.#IngressSpec & {
		if _config.ingress.className != _|_ {
			ingressClassName: _config.ingress.className
		}
		if _config.ingress.tls != _|_ {
			tls: _config.ingress.tls
		}
		if _config.ingress.hosts != _|_ {
			rules: [for h in _config.ingress.hosts {
				host: h.host
				http: paths: [for p in h.paths {
					path:     p.path
					pathType: p.pathType
					backend: service: {
						name: _config.metadata.name
						port: number: _config.service.port
					}
				}]
			}]
		}
	}
}
