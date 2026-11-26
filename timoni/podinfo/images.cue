package main

// The container images tracked from the upstream releases.
values: {
	image: {
		repository: *"ghcr.io/stefanprodan/podinfo" | string
		tag:        *"6.17.0" | string
		digest:     *"" | string
	}
}
