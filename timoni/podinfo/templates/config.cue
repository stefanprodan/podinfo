package templates

import (
	corev1 "k8s.io/api/core/v1"
	timoniv1 "timoni.sh/core/v1alpha1"
)

// Config defines the schema and defaults for the Instance values.
#Config: {
	// Podinfo optional UI setting.
	ui: {
		color:    *"#34577c" | string
		message?: string
		backend?: string
	}

	// Runtime version info automatically set at apply-time.
	moduleVersion!: string
	kubeVersion!:   string

	// The minimum Kubernetes version to 1.20.
	clusterVersion: timoniv1.#SemVer & {#Version: kubeVersion, #Minimum: "1.20.0"}

	// Kubernetes metadata common to all resources.
	metadata: timoniv1.#Metadata & {#Version: moduleVersion}

	// Label selector common to all resources.
	selector: timoniv1.#Selector & {#Name: metadata.name}

	// The number of pods replicas.
	// By default, the number of replicas is 1.
	replicas: *1 | int & >=0

	// The image allows setting the container image repository,
	// tag, digest and pull policy.
	// The default image repository and tag is set in `values.cue`.
	image!: timoniv1.#Image

	// The resources allows setting the container resource requirements.
	// By default, the container requests 10m CPU and 32Mi memory.
	resources: timoniv1.#ResourceRequirements & {
		requests: {
			cpu:    *"10m" | timoniv1.#CPUQuantity
			memory: *"32Mi" | timoniv1.#MemoryQuantity
		}
	}

	// The securityContext allows setting the container security context.
	securityContext?: corev1.#SecurityContext

	// Pod optional settings.
	podAnnotations?: {[string]: string}
	podSecurityContext?: corev1.#PodSecurityContext
	imagePullSecrets?: [...corev1.LocalObjectReference]
	tolerations?: [...corev1.#Toleration]
	topologySpreadConstraints?: [...corev1.#TopologySpreadConstraint]

	// Pod affinity rules, by default, pods are scheduled on Linux nodes.
	affinity: *{
		nodeAffinity: requiredDuringSchedulingIgnoredDuringExecution: nodeSelectorTerms: [{
			matchExpressions: [{
				key:      corev1.#LabelOSStable
				operator: "In"
				values: ["linux"]
			}]
		}]
	} | corev1.#Affinity

	// Service
	service: {
		port:         *80 | int & >0 & <=65535
		annotations?: timoniv1.#Annotations
		labels?:      timoniv1.#Labels
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
		enabled:      *false | bool
		tls:          *false | bool
		host:         *"podinfo.local" | string
		className?:   string
		annotations?: timoniv1.#Annotations
		labels?:      timoniv1.#Labels
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
		"\(config.metadata.name)-sa": #ServiceAccount & {_config: config}
		"\(config.metadata.name)-svc": #Service & {_config: config}
		"\(config.metadata.name)-deploy": #Deployment & {_config: config}

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
