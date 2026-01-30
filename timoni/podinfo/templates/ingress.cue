package templates

import (
	netv1 "k8s.io/api/networking/v1"
)

#Ingress: netv1.#Ingress & {
	_config:    #Config
	apiVersion: "networking.k8s.io/v1"
	kind:       "Ingress"
	metadata:   _config.metadata
	metadata: {
		if _config.ingress.labels != _|_ {
			labels: _config.ingress.labels
		}
		if _config.ingress.annotations != _|_ {
			annotations: _config.ingress.annotations
		}
	}
	spec: netv1.#IngressSpec & {
		rules: [{
			host: _config.ingress.host
			http: {
				paths: [{
					pathType: "Prefix"
					path:     "/"
					backend: service: {
						name: _config.metadata.name
						port: name: "http"
					}
				}]
			}
		}]
		if _config.ingress.tls {
			tls: [{
				hosts: [_config.ingress.host]
				secretName: "\(_config.metadata.name)-cert"
			}]
		}
		if _config.ingress.className != _|_ {
			ingressClassName: _config.ingress.className
		}
	}
}
