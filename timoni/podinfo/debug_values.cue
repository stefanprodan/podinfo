@if(debug)

package main

// Values used by debug_tool.cue.
// Debug example 'cue cmd -t debug -t name=podinfo -t namespace=test -t mv=1.0.0 -t kv=1.28.0 build'.
values: {
	image: {
		repository: "docker.io/stefanprodan/podinfo"
		tag:        "latest"
		digest:     ""
	}

	test: {
		enabled: true
		image: {
			repository: "ghcr.io/curl/curl-container/curl-multi"
			tag:        "master"
			digest:     ""
		}
	}

	ui: backend: "http://backend.default.svc.cluster.local/echo"

	metadata: {
		labels: "app.kubernetes.io/part-of":   "podinfo"
		annotations: "app.kubernetes.io/team": "dev"
	}

	caching: {
		enabled:  true
		redisURL: "tcp://:redis@redis:6379"
	}

	ingress: {
		enabled:   true
		className: "nginx"
		host:      "podinfo.example.com"
		tls:       true
		annotations: "cert-manager.io/cluster-issuer": "letsencrypt"
	}

	monitoring: enabled: true

	_mcpu: 100
	_mem:  128
	resources: {
		requests: {
			cpu:    "\(_mcpu)m"
			memory: "\(_mem)Mi"
		}
		limits: {
			cpu:    "\(_mcpu*2)m"
			memory: "\(_mem*2)Mi"
		}
	}

	autoscaling: {
		enabled:     true
		minReplicas: 1
		maxReplicas: 10
		cpu:         90
		memory:      "\(_mem*2-10)Mi"
	}

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
}
