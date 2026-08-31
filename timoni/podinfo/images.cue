package main

// The container images tracked from the upstream releases.
values: {
	image: {
		repository: *"ghcr.io/stefanprodan/podinfo" | string
		tag:        *"6.15.0" | string
		digest:     *"" | string
	}
	test: {
		image: {
			repository: *"ghcr.io/curl/curl-container/curl-multi" | string
			tag:        *"8.21.0" | string
			digest:     *"" | string
		}
		grpcImage: {
			repository: *"ghcr.io/grpc-ecosystem/grpc-health-probe" | string
			tag:        *"v0.4.56" | string
			digest:     *"" | string
		}
	}
}
