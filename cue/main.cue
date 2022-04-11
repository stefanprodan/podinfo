package main

import (
	podinfo "github.com/stefanprodan/podinfo/cue/podinfo"
)

resources: (podinfo.#Application & {
	input: {
		meta: {
			name: "podinfo"
			annotations: {
				"app.kubernetes.io/part-of": "podinfo"
			}
		}
		image: {
			repository: "ghcr.io/stefanprodan/podinfo"
			tag:        "6.1.1"
		}
		resources: requests: cpu: "100m"
		hpa: {
			enabled:     true
			minReplicas: 2
			maxReplicas: 4
			cpu:         99
		}
	}
}).out
