package templates

import (
	corev1 "k8s.io/api/core/v1"
	timoniv1 "timoni.sh/core/v1alpha1"
)

// Config defines the schema and defaults for the Instance values.
#Config: {
	// UI setting
	ui: {
		color:    *"#34577c" | string
		message?: string
		backend?: string
	}

	// Runtime version info
	moduleVersion!: string
	kubeVersion!:   string

	// Metadata (common to all resources)
	metadata: timoniv1.#Metadata & {#Version: moduleVersion}

	// Label selector (common to all resources)
	selector: timoniv1.#Selector & {#Name: metadata.name}

	// Deployment
	replicas: *1 | int & >=0

	// Pod
	podAnnotations?: {[ string]: string}
	podSecurityContext?: corev1.#PodSecurityContext
	imagePullSecrets?: [...corev1.LocalObjectReference]
	tolerations?: [ ...corev1.#Toleration]
	affinity?: corev1.#Affinity
	topologySpreadConstraints?: [...corev1.#TopologySpreadConstraint]

	// Container
	image:            timoniv1.#Image
	imagePullPolicy:  *"IfNotPresent" | string
	resources?:       corev1.#ResourceRequirements
	securityContext?: corev1.#SecurityContext

	// Service
	service: {
		port: *80 | int & >0 & <=65535
		annotations?: {[ string]: string}
		labels?: {[ string]: string}
	}

	// HorizontalPodAutoscaler (optional)
	autoscaling: {
		enabled:     *false | bool
		cpu:         *99 | int & >0 & <=100
		memory:      *"" | string
		minReplicas: *replicas | int
		maxReplicas: *minReplicas | int & >=minReplicas
	}

	// Ingress (optional)
	ingress: {
		enabled: *false | bool
		tls:     *false | bool
		host:    *"podinfo.local" | string
		annotations?: {[ string]: string}
		labels?: {[ string]: string}
		className?: string
	}

	// ServiceMonitor (optional)
	monitoring: {
		enabled:  *false | bool
		interval: *15 | int & >=5 & <=3600
	}

	// Caching (optional)
	caching: {
		enabled:   *false | bool
		redisURL?: string & =~"^tcp://.*$"
	}

	// Test Jobs (optional)
	test: {
		enabled: *false | bool
		image!:  timoniv1.#Image
	}
}

// Instance takes the config values and outputs the Kubernetes objects.
#Instance: {
	config: #Config

	objects: {
		"\(config.metadata.name)-sa":     #ServiceAccount & {_config: config}
		"\(config.metadata.name)-svc":    #Service & {_config:        config}
		"\(config.metadata.name)-deploy": #Deployment & {_config:     config}

		if config.autoscaling.enabled {
			"\(config.metadata.name)-hpa": #HorizontalPodAutoscaler & {_config: config}
		}

		if config.ingress.enabled {
			"\(config.metadata.name)-ingress": #Ingress & {_config: config}
		}

		if config.monitoring.enabled {
			"\(config.metadata.name)-monitor": #ServiceMonitor & {_config: config}
		}
	}

	tests: {
		"test-svc": #TestJob & {_config: config}
	}
}
