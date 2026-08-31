@if(debug)

package main

// Values used by `timoni mod vet --debug` and `debug_tool.cue`.
// They enable every optional object so that all templates are
// validated against their Kubernetes and CRD schemas.
// Debug example 'cue cmd -t debug -t name=podinfo -t namespace=test -t mv=1.0.0 -t kv=1.28.0 build'.
values: {
	image: {
		repository: "docker.io/stefanprodan/podinfo"
		tag:        "latest"
		digest:     ""
	}

	test: enabled: true

	// The default vet covers the hardened profile; flip to platform so
	// both identity branches validate.
	securityContextPreset: "platform"
	replicas:              2
	revisionHistoryLimit:  5
	logLevel:              "debug"

	strategy: {
		type: "RollingUpdate"
		rollingUpdate: {
			maxUnavailable: 1
			maxSurge:       "25%"
		}
	}

	metadata: {
		labels: "app.kubernetes.io/part-of":   "podinfo"
		annotations: "app.kubernetes.io/team": "dev"
	}

	podLabels: "team":                      "dev"
	podAnnotations: "prometheus.io/scrape": "false"

	ui: {
		message: "Hello from Timoni"
		backend: "http://backend.default.svc.cluster.local/echo"
	}

	caching: {
		enabled:  true
		redisURL: "tcp://:redis@redis:6379"
	}

	extraArgs: ["--random-delay=true"]

	env: [{
		name:  "PODINFO_UI_LOGO"
		value: "https://raw.githubusercontent.com/stefanprodan/podinfo/gh-pages/cuddle_clap.gif"
	}]

	extraContainers: [{
		name:  "sidecar"
		image: "docker.io/library/busybox:latest"
		command: ["sleep", "infinity"]
	}]
	initContainers: [{
		name:  "init"
		image: "docker.io/library/busybox:latest"
		command: ["true"]
	}]

	extraVolumes: [{
		name: "cache"
		emptyDir: {}
	}]
	extraVolumeMounts: [{
		name:      "cache"
		mountPath: "/cache"
	}]

	startupProbe: {
		httpGet: {
			path: "/healthz"
			port: "http"
		}
		failureThreshold: 30
		periodSeconds:    2
	}

	terminationGracePeriodSeconds: 30
	priorityClassName:             "system-cluster-critical"
	schedulerName:                 "default-scheduler"

	nodeSelector: "kubernetes.io/os": "linux"
	tolerations: [{
		key:      "node-role.kubernetes.io/control-plane"
		operator: "Exists"
		effect:   "NoSchedule"
	}]
	topologySpreadConstraints: [{
		maxSkew:           1
		topologyKey:       "topology.kubernetes.io/zone"
		whenUnsatisfiable: "ScheduleAnyway"
		labelSelector: matchLabels: "app.kubernetes.io/name": "podinfo"
	}]
	affinity: podAntiAffinity: "hard"

	hostAliases: [{
		ip: "10.0.0.10"
		hostnames: ["backend.internal"]
	}]
	dnsConfig: options: [{
		name:  "ndots"
		value: "2"
	}]

	deploymentAnnotations: "team": "dev"

	podSecurityContext: fsGroup: 101

	serviceAccount: {
		annotations: "eks.amazonaws.com/role-arn": "arn:aws:iam::111122223333:role/podinfo"
	}

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

	hpa: {
		enabled:     true
		minReplicas: 1
		maxReplicas: 10
		cpu:         90
		memory:      "\(_mem*2-10)Mi"
		behavior: scaleDown: stabilizationWindowSeconds: 300
	}

	podDisruptionBudget: {
		enabled:                    true
		minAvailable:               1
		unhealthyPodEvictionPolicy: "AlwaysAllow"
	}

	service: {
		type: "NodePort"
		port: 8080
		labels: "kubernetes.io/cluster-service": "true"
		nodePort: 30898
		ipFamilies: ["IPv4", "IPv6"]
		ipFamilyPolicy:        "PreferDualStack"
		externalTrafficPolicy: "Local"
	}

	ingress: {
		enabled:   true
		className: "nginx"
		annotations: "cert-manager.io/cluster-issuer": "letsencrypt"
		hosts: [{
			host: "podinfo.example.com"
			paths: [{path: "/", pathType: "Prefix"}]
		}]
		tls: [{
			secretName: "podinfo-ingress-tls"
			hosts: ["podinfo.example.com"]
		}]
	}

	httpRoute: {
		enabled: true
		parentRefs: [{
			name:      "gateway"
			namespace: "gateway-system"
		}]
		hostnames: ["podinfo.example.com"]
		rules: [{
			matches: [{path: {type: "PathPrefix", value: "/"}}]
			filters: [{
				type: "RequestHeaderModifier"
				requestHeaderModifier: set: [{
					name:  "X-Forwarded-Proto"
					value: "https"
				}]
			}]
		}]
	}

	serviceMonitor: {
		enabled: true
		additionalLabels: prometheus: "platform"
		annotations: "team":          "dev"
		interval:      "1m30s"
		scrapeTimeout: "5s"
		honorLabels:   true
		sampleLimit:   1000
		targetLabels: ["app.kubernetes.io/part-of"]
		metricRelabelings: [{
			action: "drop"
			sourceLabels: ["__name__"]
			regex: "go_gc_.*"
		}]
	}
}
