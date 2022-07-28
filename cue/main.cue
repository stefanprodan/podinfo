package main

import (
	podinfo "github.com/stefanprodan/podinfo/cue/podinfo"
)

app: podinfo.#Application & {
	config: {
		meta: {
			name:      "podinfo"
			namespace: "default"
		}
		image: tag: "6.1.8"
		resources: requests: {
			cpu:    "100m"
			memory: "16Mi"
		}
		hpa: {
			enabled:     true
			maxReplicas: 3
		}
		ingress: {
			enabled:   true
			className: "nginx"
			host:      "podinfo.example.com"
			tls:       true
			annotations: "cert-manager.io/cluster-issuer": "letsencrypt"
		}
		serviceMonitor: enabled: true
	}
}

objects: app.objects
