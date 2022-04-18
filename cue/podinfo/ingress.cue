package podinfo

import (
	netv1 "k8s.io/api/networking/v1"
)

#ingressConfig: {
	enabled: *false | bool
	annotations?: {[ string]: string}
	className?: string
	tls:        *false | bool
	host:       string
}

#Ingress: netv1.#Ingress & {
	_config:    #Config
	apiVersion: "networking.k8s.io/v1"
	kind:       "Ingress"
	metadata:   _config.meta
	if _config.ingress.annotations != _|_ {
		metadata: annotations: _config.ingress.annotations
	}
	spec: netv1.#IngressSpec & {
		rules: [{
			host: _config.ingress.host
			http: {
				paths: [{
					pathType: "Prefix"
					path:     "/"
					backend: service: {
						name: _config.meta.name
						port: name: "http"
					}
				}]
			}
		}]
		if _config.ingress.tls {
			tls: [{
				hosts: [_config.ingress.host]
				secretName: "\(_config.meta.name)-cert"
			}]
		}
		if _config.ingress.className != _|_ {
			ingressClassName: _config.ingress.className
		}
	}
}
