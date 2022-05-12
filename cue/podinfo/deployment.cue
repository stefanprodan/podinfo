package podinfo

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

#Deployment: appsv1.#Deployment & {
	_config:         #Config
	_serviceAccount: string
	apiVersion:      "apps/v1"
	kind:            "Deployment"
	metadata:        _config.meta
	spec:            appsv1.#DeploymentSpec & {
		if !_config.hpa.enabled {
			replicas: _config.replicas
		}
		strategy: {
			type: "RollingUpdate"
			rollingUpdate: maxUnavailable: 1
		}
		selector: matchLabels: _config.selectorLabels
		template: {
			metadata: {
				labels: _config.selectorLabels
				if !_config.serviceMonitor.enabled {
					annotations: {
						"prometheus.io/scrape": "true"
						"prometheus.io/port":   "\(_config.service.metricsPort)"
					}
				}
			}
			spec: corev1.#PodSpec & {
				terminationGracePeriodSeconds: 15
				serviceAccountName:            _serviceAccount
				containers: [
					{
						name:            "podinfo"
						image:           "\(_config.image.repository):\(_config.image.tag)"
						imagePullPolicy: _config.image.pullPolicy
						command: [
							"./podinfo",
							"--port=\(_config.service.httpPort)",
							"--port-metrics=\(_config.service.metricsPort)",
							"--grpc-port=\(_config.service.grpcPort)",
							"--level=\(_config.logLevel)",
							if _config.cache != _|_ {
								"--cache-server=\(_config.cache)"
							},
							for b in _config.backends {
								"--backend-url=\(b)"
							},
						]
						ports: [
							{
								name:          "http"
								containerPort: _config.service.httpPort
								protocol:      "TCP"
							},
							{
								name:          "http-metrics"
								containerPort: _config.service.metricsPort
								protocol:      "TCP"
							},
							{
								name:          "grpc"
								containerPort: _config.service.grpcPort
								protocol:      "TCP"
							},
						]
						livenessProbe: {
							httpGet: {
								path: "/healthz"
								port: "http"
							}
						}
						readinessProbe: {
							httpGet: {
								path: "/readyz"
								port: "http"
							}
						}
						volumeMounts: [
							{
								name:      "data"
								mountPath: "/data"
							},
						]
						resources: _config.resources
						if _config.securityContext != _|_ {
							securityContext: _config.securityContext
						}
					},
				]
				if _config.affinity != _|_ {
					affinity: _config.affinity
				}
				if _config.tolerations != _|_ {
					tolerations: _config.tolerations
				}
				volumes: [
					{
						name: "data"
						emptyDir: {}
					},
				]
			}
		}
	}
}
