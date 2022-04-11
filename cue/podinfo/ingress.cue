package podinfo

import (
	netv1 "k8s.io/api/networking/v1"
)

#ingressConfig: {
	svcName:   string
	svcPort:   int
	enabled:   *false | bool
	className: *"" | string
	tls: [{
		hosts: [string]
		secretName: string
	}]
	hosts: [{
		host: "podinfo.local"
		paths: [{
			path:     "/"
			pathType: "ImplementationSpecific"
		}]
	}]
}

#Ingress: netv1.#Ingress & {
	_config:    #Config
	apiVersion: "networking.k8s.io/v1"
	kind:       "Ingress"
	metadata:   _config.meta
	spec:       netv1.#IngressSpec & {
		ingressClassName: _config.ingress.className
		tls: [ for t in _config.ingress.tls {
			hosts:      t.hosts
			secretName: t.secretName
		}]
		rules: [ for h in _config.ingress.hosts {
			host: h.host
			http: paths: [ for p in h.paths {
				path:     p.path
				pathType: p.pathType
				backend: service: {
					name: _config.meta.name
					port: number: _config.service.externalPort
				}
			}]
		}]
	}
}
