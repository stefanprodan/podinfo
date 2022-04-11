package podinfo

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
)

#Config: {
	meta: metav1.#ObjectMeta
	image: {
		repository: *"ghcr.io/stefanprodan/podinfo" | string
		tag:        string
		pullPolicy: *"IfNotPresent" | string
	}
	replicas: *1 | int
	service:  #serviceConfig
	host:     string
	cache:    string
	backends: [string]
	logLevel: *"info" | string
	faults: {
		delay:     *false | bool
		error:     *false | bool
		unhealthy: *false | bool
		unready:   *false | bool
	}
	h2c: {
		enabled: *false | bool
	}
	ui: {
		color:   *"#34577c" | string
		message: *"" | string
		logo:    *"" | string
	}
	podAnnotations: {[ string]: string}
	securityContext: corev1.#PodSecurityContext
	resources:       *{
		requests: {
			cpu:    "1m"
			memory: "16Mi"
		}
	} | corev1.#ResourceRequirements
	nodeSelector: {[ string]: string}
	affinity: corev1.#Affinity
	tolerations: [ ...corev1.#Toleration]
	linkerd: {
		enabled: *false | bool
	}
	redis: {
		enabled: *false | bool
	}
	tls: {
		enabled:    *false | bool
		port:       *9899 | int
		certPath:   *"/data/cert" | string
		secretName: *"" | string
	}
	cert:           #certConfig
	hpa:            #hpaConfig
	ingress:        #ingressConfig
	serviceMonitor: #serviceMonConfig
}
