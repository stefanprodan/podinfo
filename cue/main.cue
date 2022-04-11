package main

import (
	podinfo "github.com/stefanprodan/podinfo/cue/podinfo"
)

resources: (podinfo.#Application & {
	input: {
		meta: {
			name: "podinfo"
			annotations: {
				"app.kubernetes.io/name": "podinfo"
			}
		}
		image: {
			repository: "ghcr.io/stefanprodan/podinfo"
			tag:        "6.1.1"
		}
		service: {
			grpcPort: 6666
		}
	}
}).out
