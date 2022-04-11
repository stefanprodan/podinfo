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
		if _config.hpa.enabled == false {
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
				annotations: {
					"prometheus.io/scrape": "true"
					"prometheus.io/port":   "\(_config.service.metricsPort)"
					_config.podAnnotations
				}
			}
			spec: corev1.#PodSpec & {
				terminationGracePeriodSeconds: 30
				serviceAccountName:            _serviceAccount
				containers: [
					{
						name:            "podinfo"
						image:           "\(_config.image.repository):\(_config.image.tag)"
						imagePullPolicy: _config.image.pullPolicy
						securityContext: _config.securityContext
						command: [
							"./podinfo",
							"--port=\(_config.service.httpPort)",
							"--port-metrics=\(_config.service.metricsPort)",
							"--grpc-port=\(_config.service.grpcPort)",
							"--level=\(_config.logLevel)",
							"--random-delay=\(_config.faults.delay)",
							"--random-error=\(_config.faults.error)",
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
							exec: {
								command: [
									"podcli",
									"check",
									"http",
									"localhost:\(_config.service.httpPort)/healthz",
								]
							}
							initialDelaySeconds: 1
							timeoutSeconds:      5
						}
						readinessProbe: {
							exec: {
								command: [
									"podcli",
									"check",
									"http",
									"localhost:\(_config.service.httpPort)/readyz",
								]
							}
							initialDelaySeconds: 1
							timeoutSeconds:      5
						}
						volumeMounts: [
							{
								name:      "data"
								mountPath: "/data"
							},
							if _config.tls.secretName != "" {
								name:      "tls"
								mountPath: _config.tls.certPath
								readOnly:  true
							},
						]
						resources: _config.resources
					},
				]
				nodeSelector: _config.nodeSelector
				affinity:     _config.affinity
				tolerations:  _config.tolerations
				volumes: [
					{
						name: "data"
						emptyDir: {}
					},
					if _config.tls.secretName != "" {
						name: "tls"
						secret: {
							secretName: _config.tls.secretName
						}
					},
				]
			}
		}
	}
}
