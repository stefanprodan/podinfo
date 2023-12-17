package templates

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

#Deployment: appsv1.#Deployment & {
	_config:    #Config
	apiVersion: "apps/v1"
	kind:       "Deployment"
	metadata:   _config.metadata
	spec: appsv1.#DeploymentSpec & {
		if !_config.autoscaling.enabled {
			replicas: _config.replicas
		}
		strategy: {
			type: "RollingUpdate"
			rollingUpdate: maxUnavailable: "50%"
		}
		selector: matchLabels: _config.selector.labels
		template: {
			metadata: {
				labels: _config.selector.labels
				if _config.podAnnotations != _|_ {
					annotations: _config.podAnnotations
				}
				if !_config.monitoring.enabled {
					annotations: {
						"prometheus.io/scrape": "true"
						"prometheus.io/port":   "9797"
					}
				}
			}
			spec: corev1.#PodSpec & {
				serviceAccountName: _config.metadata.name
				containers: [
					{
						name:            _config.metadata.name
						image:           _config.image.reference
						imagePullPolicy: _config.image.pullPolicy
						ports: [
							{
								name:          "http"
								containerPort: 9898
								protocol:      "TCP"
							},
							{
								name:          "http-metrics"
								containerPort: 9797
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
						if _config.resources != _|_ {
							resources: _config.resources
						}
						if _config.securityContext != _|_ {
							securityContext: _config.securityContext
						}
						env: [
							{
								name:  "PODINFO_UI_COLOR"
								value: _config.ui.color
							},
							if _config.ui.message != _|_ {
								{
									name:  "PODINFO_UI_MESSAGE"
									value: _config.ui.message
								}
							},
							if _config.ui.backend != _|_ {
								{
									name:  "PODINFO_BACKEND_URL"
									value: _config.ui.backend
								}
							},
						]
						command: [
							"./podinfo",
							"--level=info",
							"--port=9898",
							"--port-metrics=9797",
							if _config.caching.enabled {
								"--cache-server=\(_config.caching.redisURL)"
							},
						]
						volumeMounts: [
							{
								name:      "data"
								mountPath: "/data"
							},
						]
					},
				]
				if _config.podSecurityContext != _|_ {
					securityContext: _config.podSecurityContext
				}
				if _config.topologySpreadConstraints != _|_ {
					topologySpreadConstraints: _config.topologySpreadConstraints
				}
				if _config.affinity != _|_ {
					affinity: _config.affinity
				}
				if _config.tolerations != _|_ {
					tolerations: _config.tolerations
				}
				if _config.imagePullSecrets != _|_ {
					imagePullSecrets: _config.imagePullSecrets
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
