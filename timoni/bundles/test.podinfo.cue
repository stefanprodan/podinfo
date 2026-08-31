bundle: {
	apiVersion: "v1alpha1"
	name:       "podinfo"

	_modURL: "oci://ghcr.io/stefanprodan/modules/podinfo" @timoni(runtime:string:PODINFO_MODULE_URL)
	_imgURL: "ghcr.io/stefanprodan/modules/podinfo"       @timoni(runtime:string:PODINFO_IMAGE_URL)
	_imgTag: "latest"                                     @timoni(runtime:string:PODINFO_VERSION)

	instances: {
		backend: {
			module: url: _modURL
			namespace: "podinfo"
			values: {
				image: {
					repository: _imgURL
					tag:        _imgTag
				}
				resources: requests: {
					cpu:    "100m"
					memory: "128Mi"
				}
				hpa: {
					enabled:     true
					minReplicas: 1
					maxReplicas: 10
					cpu:         90
				}
				test: enabled: true
			}
		}
		frontend: {
			module: url: _modURL
			namespace: "podinfo"
			values: {
				image: {
					repository: _imgURL
					tag:        _imgTag
				}
				ui: backend: "http://backend.podinfo.svc.cluster.local/echo"
				replicas: 2
				podDisruptionBudget: enabled: true
				// The e2e cluster runs no ingress controller; the Ingress
				// exercises the template with a controller-agnostic config.
				ingress: {
					enabled: true
					hosts: [{host: "podinfo.local"}]
					tls: [{
						secretName: "podinfo-cert"
						hosts: ["podinfo.local"]
					}]
				}
				test: enabled: true
			}
		}
	}
}
