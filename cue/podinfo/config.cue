package podinfo

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
)

#Config: {
	meta:           metav1.#ObjectMeta
	hpa:            #hpaConfig
	ingress:        #ingressConfig
	service:        #serviceConfig
	serviceMonitor: #serviceMonConfig

	image: {
		repository: *"ghcr.io/stefanprodan/podinfo" | string
		pullPolicy: *"IfNotPresent" | string
		tag:        string
	}

	cache?: string & =~"^tcp://"
	backends: [...string]
	logLevel: *"info" | string
	replicas: *1 | int

	resources: *{
		requests: {
			cpu:    "1m"
			memory: "16Mi"
		}
		limits: memory: "128Mi"
	} | corev1.#ResourceRequirements

	selectorLabels: *{"app.kubernetes.io/name": meta.name} | {[ string]: string}
	meta: annotations: *{"app.kubernetes.io/version": "\(image.tag)"} | {[ string]: string}
	meta: labels:      *selectorLabels | {[ string]:  string}

	securityContext?: corev1.#PodSecurityContext
	affinity?:        corev1.#Affinity
	tolerations?: [ ...corev1.#Toleration]
}
