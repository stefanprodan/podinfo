package templates

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	timoniv1 "timoni.sh/core/v1alpha1"
)

// Config defines the schema and defaults for the Instance values.
#Config: {
	// Runtime version info automatically set at apply-time.
	moduleVersion!: string
	kubeVersion!:   string

	// The minimum Kubernetes version is 1.25.
	clusterVersion: timoniv1.#SemVer & {#Version: kubeVersion, #Minimum: "1.25.0"}

	// Kubernetes metadata common to all resources.
	// The `metadata.name` and `metadata.namespace` fields are
	// set from the user-supplied instance name and namespace.
	metadata: timoniv1.#Metadata & {#Version: moduleVersion}

	// Extra labels and annotations added to all resources.
	// The `app.kubernetes.io/name` and `app.kubernetes.io/version`
	// labels are generated from the instance name and module version.
	metadata: labels:       timoniv1.#Labels
	metadata: annotations?: timoniv1.#Annotations

	// Label selector common to all resources.
	selector: timoniv1.#Selector & {#Name: metadata.name}

	// Podinfo UI settings: the background color, the greeting
	// message and the URL of the backend service called
	// by the `/api/echo` endpoint.
	ui: {
		color:    *"#34577c" | string
		message?: string
		backend?: string
	}

	// The log level of the podinfo container.
	logLevel: *"info" | "debug" | "warn" | "error"

	// Redis caching (optional); the Redis URL in the
	// format `tcp://[:password@]host[:port]` is required when enabled.
	caching: {
		enabled: *false | bool
		if enabled {
			redisURL: string & =~"^tcp://.*$"
		}
		redisURL?: string & =~"^tcp://.*$"
	}

	// The number of pod replicas; ignored when the autoscaler is enabled.
	replicas: *1 | int & >=0

	// The number of old ReplicaSets to retain.
	revisionHistoryLimit?: int & >=0

	// The strategy to replace old pods with new ones. By default,
	// half of the pods can be unavailable during a rolling update.
	strategy: *{
		type: "RollingUpdate"
		rollingUpdate: maxUnavailable: "50%"
	} | appsv1.#DeploymentStrategy

	// The container image repository, tag, digest and pull policy.
	// The default repository, tag and digest are set in `images.cue`.
	image!: timoniv1.#Image

	// References to secrets used for pulling images from private registries.
	imagePullSecrets?: [...timoniv1.#ObjectReference]

	// Extra command line arguments appended to the podinfo command.
	extraArgs: *[] | [...string]

	// Extra environment variables of the podinfo container.
	env?: [...corev1.#EnvVar]

	// The container resource requirements.
	// By default, the container requests 10m CPU and 32Mi memory.
	resources: timoniv1.#ResourceRequirements & {
		requests: {
			cpu:    *"10m" | timoniv1.#CPUQuantity
			memory: *"32Mi" | timoniv1.#MemoryQuantity
		}
	}

	// The container security context, hardened by default: privilege
	// escalation is denied, the root filesystem is read-only and all
	// capabilities are dropped. The pod identity (UID/GID) is a
	// pod-level concern, see podSecurityContext.
	securityContext: corev1.#SecurityContext & timoniv1.#ContainerSecurityContext

	// The liveness probe of the podinfo container.
	livenessProbe: corev1.#Probe & {
		httpGet: {
			path: *"/healthz" | string
			port: *"http" | string | int
		}
	}

	// The readiness probe of the podinfo container.
	readinessProbe: corev1.#Probe & {
		httpGet: {
			path: *"/readyz" | string
			port: *"http" | string | int
		}
	}

	// The startup probe of the podinfo container (optional).
	startupProbe?: corev1.#Probe

	// The volume backing /data, where podinfo stores its runtime
	// data (the root filesystem is read-only).
	dataVolume: *{emptyDir: {}} | corev1.#VolumeSource

	// Extra volumes and volume mounts of the podinfo container.
	extraVolumes?: [...corev1.#Volume]
	extraVolumeMounts?: [...corev1.#VolumeMount]

	// Extra containers and init containers added to the pods.
	extraContainers?: [...corev1.#Container]
	initContainers?: [...corev1.#Container]

	// The security preset applied to the pod identity defaults: the
	// default "hardened" preset pins the image's non-root UID/GID,
	// while "platform" leaves the identity to an admission controller
	// (e.g. an OpenShift SecurityContextConstraint).
	securityContextPreset: timoniv1.#SecurityContextPreset

	// The pod security context generated for the security preset.
	// Under the hardened preset, the identity defaults are pinned to
	// the non-root user of the podinfo image (UID 100, GID 101).
	podSecurityContext: corev1.#PodSecurityContext & timoniv1.#PodSecurityContext & {
		#Preset:  securityContextPreset
		#User:    100
		#Group:   101
		#FSGroup: 101
	}

	// Pod optional settings.
	podLabels?:      timoniv1.#Labels
	podAnnotations?: timoniv1.#Annotations
	// Pods are scheduled on Linux nodes by default.
	nodeSelector: *{"kubernetes.io/os": "linux"} | {[string]: string}
	tolerations?: [...corev1.#Toleration]
	topologySpreadConstraints?: [...corev1.#TopologySpreadConstraint]
	hostAliases?: [...corev1.#HostAlias]
	dnsConfig?:                     corev1.#PodDNSConfig
	dnsPolicy?:                     "ClusterFirst" | "ClusterFirstWithHostNet" | "Default" | "None"
	priorityClassName?:             string & =~".+"
	schedulerName?:                 string & =~".+"
	terminationGracePeriodSeconds?: int & >=0

	// Mount the service account token into the pod; podinfo does not
	// talk to the Kubernetes API, so the token is not mounted by default.
	automountServiceAccountToken: *false | bool

	// The affinity rules; Linux placement comes from the nodeSelector
	// default. `podAntiAffinity` accepts the `soft` (default), `hard`
	// and `none` presets for spreading the replicas across nodes, or
	// raw pod anti-affinity rules.
	affinity: timoniv1.#AffinityValues & {
		podAntiAffinity: timoniv1.#AffinityPreset | corev1.#PodAntiAffinity
		nodeAffinity?:   corev1.#NodeAffinity
		podAffinity?:    corev1.#PodAffinity
	}

	// Labels and annotations added to the Deployment.
	deploymentLabels?:      timoniv1.#Labels
	deploymentAnnotations?: timoniv1.#Annotations

	// ServiceAccount settings. Set `create: false` to use an existing
	// service account referenced by `name`.
	serviceAccount: {
		create: *true | bool
		if create {
			name: *metadata.name | string
		}
		if !create {
			name: *"default" | string
		}
		labels?:      timoniv1.#Labels
		annotations?: timoniv1.#Annotations
		// The token is mounted through the pod setting instead.
		automountServiceAccountToken: *false | bool
	}

	// Service settings. The http port (80 by default) targets the
	// podinfo HTTP port, the grpc port (9999 by default) targets the
	// podinfo gRPC port and the http-metrics port (9797) exposes
	// the Prometheus metrics endpoint.
	service: {
		type:       *"ClusterIP" | "NodePort" | "LoadBalancer"
		port:       *80 | int & >0 & <=65535
		grpcPort:   *9999 | int & >0 & <=65535
		clusterIP?: string & =~".+"
		ipFamilies?: [..."IPv4" | "IPv6"]
		ipFamilyPolicy?: "SingleStack" | "PreferDualStack" | "RequireDualStack"
		externalIPs?: [...string]
		if type != "ClusterIP" {
			// Zero lets the cluster assign the node ports.
			nodePort:               *0 | int & >=0 & <=32767
			grpcNodePort:           *0 | int & >=0 & <=32767
			externalTrafficPolicy?: "Cluster" | "Local"
		}
		if type == "LoadBalancer" {
			loadBalancerIP?:    string & =~".+"
			loadBalancerClass?: string & =~".+"
			loadBalancerSourceRanges?: [...string]
		}
		annotations?: timoniv1.#Annotations
		labels?:      timoniv1.#Labels
	}

	// Ingress for the podinfo http Service port (optional).
	ingress: {
		enabled:    *false | bool
		className?: string & =~".+"
		if enabled {
			hosts: [#IngressHost, ...#IngressHost]
		}
		tls?: [...networkingv1.#IngressTLS]
		annotations?: timoniv1.#Annotations
		labels?:      timoniv1.#Labels
	}

	// Gateway API HTTPRoute for the podinfo http Service port
	// (optional), an alternative to the Ingress. The route backend is
	// generated by the module; each rule takes matches and filters.
	httpRoute: {
		enabled: *false | bool
		if enabled {
			parentRefs: [{...}, ...{...}]
		}
		hostnames?: [...string & =~".+"]
		rules: *[{matches: [{path: {type: "PathPrefix", value: "/"}}]}] | [...{
			matches?: [...{...}]
			filters?: [...{...}]
		}]
		annotations?: timoniv1.#Annotations
		labels?:      timoniv1.#Labels
	}

	// PodDisruptionBudget (optional). The mutually exclusive
	// `minAvailable` and `maxUnavailable` accept an absolute number
	// or a percentage; `minAvailable: 1` is the default.
	// `unhealthyPodEvictionPolicy` requires Kubernetes 1.27 or newer
	// and is omitted on older clusters.
	podDisruptionBudget: {
		enabled:                     *false | bool
		unhealthyPodEvictionPolicy?: "IfHealthyBudget" | "AlwaysAllow"
		*{minAvailable: *1 | int & >=0 | string & =~"^[0-9]+%$"} | {maxUnavailable: int & >=0 | string & =~"^[0-9]+%$"}
	}

	// HorizontalPodAutoscaler (optional). When enabled, the Deployment
	// leaves the replica count to the autoscaler, which scales the pods
	// on the CPU average utilization (percentage, 0 disables it) and,
	// when set, on the memory average value. Extra scaling metrics are
	// appended to the generated ones.
	hpa: {
		enabled:     *false | bool
		minReplicas: *1 | int & >0
		maxReplicas: *minReplicas | int & >=minReplicas
		cpu:         *99 | int & >=0 & <=100
		memory:      *"" | timoniv1.#MemoryQuantity
		metrics: *[] | [...]
		behavior?: {...}
	}

	// Prometheus Operator ServiceMonitor (optional), created in the
	// instance namespace and scraping the http-metrics Service port.
	// When disabled, the metrics endpoint is advertised to Prometheus
	// through the `prometheus.io/scrape` pod annotations.
	serviceMonitor: timoniv1.#MonitorValues

	// Test Jobs (optional), run by Timoni after the deployment becomes
	// ready: a curl Job verifying that the podinfo http Service port is
	// reachable from inside the cluster, and a grpc-health-probe Job
	// verifying the gRPC health service. The default images are set
	// in `images.cue`.
	test: {
		enabled:    *false | bool
		image!:     timoniv1.#Image
		grpcImage!: timoniv1.#Image
	}
}

// IngressHost defines the schema for an Ingress rule of one host.
#IngressHost: {
	host: string & =~".+"
	paths: *[{path: "/", pathType: "Prefix"}] | [...{
		path:     string & =~".+"
		pathType: *"Prefix" | "Exact" | "ImplementationSpecific"
	}]
}

// Instance takes the config values and outputs the Kubernetes objects.
#Instance: {
	config: #Config

	objects: {
		if config.serviceAccount.create {
			"\(config.metadata.name)-sa": #ServiceAccount & {_config: config}
		}

		"\(config.metadata.name)-svc": #Service & {_config: config}
		"\(config.metadata.name)-deploy": #Deployment & {_config: config}

		if config.hpa.enabled {
			"\(config.metadata.name)-hpa": #HorizontalPodAutoscaler & {_config: config}
		}

		if config.podDisruptionBudget.enabled {
			"\(config.metadata.name)-pdb": #PodDisruptionBudget & {_config: config}
		}

		if config.ingress.enabled {
			"\(config.metadata.name)-ingress": #Ingress & {_config: config}
		}

		if config.httpRoute.enabled {
			"\(config.metadata.name)-httproute": #HTTPRoute & {_config: config}
		}

		if config.serviceMonitor.enabled {
			"\(config.metadata.name)-monitor": #ServiceMonitor & {_config: config}
		}
	}

	tests: {
		"test-svc": #TestJob & {_config: config}
		"test-grpc": #TestGRPCJob & {_config: config}
	}
}
