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
				autoscaling: {
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
				podSecurityContext: {
					runAsUser:  100
					runAsGroup: 101
					fsGroup:    101
				}
				securityContext: {
					allowPrivilegeEscalation: false
					readOnlyRootFilesystem:   true
					runAsNonRoot:             true
					capabilities: drop: ["ALL"]
					seccompProfile: type: "RuntimeDefault"
				}
				ingress: {
					enabled:   true
					className: "nginx"
					host:      "podinfo.local"
					tls:       true
					annotations: {
						"nginx.ingress.kubernetes.io/ssl-redirect":       "false"
						"nginx.ingress.kubernetes.io/force-ssl-redirect": "false"
						"cert-manager.io/cluster-issuer":                 "self-signed"
					}
				}
				test: enabled: true
			}
		}
	}
}
