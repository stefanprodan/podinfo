package templates

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	timoniv1 "timoni.sh/core/v1alpha1"
)

#Deployment: appsv1.#Deployment & {
	_config: #Config

	// The affinity rules generated from the affinity values;
	// the anti-affinity presets match the instance selector labels.
	_affinity: timoniv1.#Affinity & {
		#Values:      _config.affinity
		#MatchLabels: _config.selector.labels
	}

	apiVersion: "apps/v1"
	kind:       "Deployment"
	metadata:   _config.metadata
	if _config.deploymentLabels != _|_ {
		metadata: labels: _config.deploymentLabels
	}
	if _config.deploymentAnnotations != _|_ {
		metadata: annotations: _config.deploymentAnnotations
	}

	spec: appsv1.#DeploymentSpec & {
		// With the autoscaler enabled, the replica count belongs to it.
		if !_config.hpa.enabled {
			replicas: _config.replicas
		}
		if _config.revisionHistoryLimit != _|_ {
			revisionHistoryLimit: _config.revisionHistoryLimit
		}
		strategy: _config.strategy
		selector: matchLabels: _config.selector.labels
		template: {
			metadata: {
				labels: _config.selector.labels
				if _config.podLabels != _|_ {
					labels: _config.podLabels
				}

				// Without a ServiceMonitor, the metrics endpoint is
				// advertised to Prometheus through pod annotations.
				if !_config.serviceMonitor.enabled {
					annotations: {
						"prometheus.io/scrape": "true"
						"prometheus.io/port":   "9797"
					}
				}
				if _config.podAnnotations != _|_ {
					annotations: _config.podAnnotations
				}
			}
			spec: corev1.#PodSpec & {
				serviceAccountName:           _config.serviceAccount.name
				automountServiceAccountToken: _config.automountServiceAccountToken
				if _config.priorityClassName != _|_ {
					priorityClassName: _config.priorityClassName
				}
				if _config.schedulerName != _|_ {
					schedulerName: _config.schedulerName
				}
				if _config.terminationGracePeriodSeconds != _|_ {
					terminationGracePeriodSeconds: _config.terminationGracePeriodSeconds
				}
				if _config.hostAliases != _|_ {
					hostAliases: _config.hostAliases
				}
				if _config.dnsConfig != _|_ {
					dnsConfig: _config.dnsConfig
				}
				if _config.dnsPolicy != _|_ {
					dnsPolicy: _config.dnsPolicy
				}
				if _config.imagePullSecrets != _|_ {
					imagePullSecrets: _config.imagePullSecrets
				}
				securityContext: _config.podSecurityContext
				nodeSelector:    _config.nodeSelector
				if _affinity.#Enabled {
					affinity: _affinity
				}
				if _config.tolerations != _|_ {
					tolerations: _config.tolerations
				}
				if _config.topologySpreadConstraints != _|_ {
					topologySpreadConstraints: _config.topologySpreadConstraints
				}
				if _config.initContainers != _|_ {
					initContainers: _config.initContainers
				}
				containers: [
					{
						name:            _config.metadata.name
						image:           _config.image.reference
						imagePullPolicy: _config.image.pullPolicy
						securityContext: _config.securityContext
						command: [
							"./podinfo",
							"--level=\(_config.logLevel)",
							"--port=9898",
							"--port-metrics=9797",
							"--grpc-port=9999",
							if _config.caching.enabled {
								"--cache-server=\(_config.caching.redisURL)"
							},
							for a in _config.extraArgs {a},
						]
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
							if _config.env != _|_ for e in _config.env {e},
						]
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
							{
								name:          "grpc"
								containerPort: 9999
								protocol:      "TCP"
							},
						]
						livenessProbe:  _config.livenessProbe
						readinessProbe: _config.readinessProbe
						if _config.startupProbe != _|_ {
							startupProbe: _config.startupProbe
						}
						resources: _config.resources
						volumeMounts: [
							{
								name:      "data"
								mountPath: "/data"
							},
							if _config.extraVolumeMounts != _|_ for m in _config.extraVolumeMounts {m},
						]
					},
					if _config.extraContainers != _|_ for c in _config.extraContainers {c},
				]
				volumes: [
					{
						name: "data"
						_config.dataVolume
					},
					if _config.extraVolumes != _|_ for v in _config.extraVolumes {v},
				]
			}
		}
	}
}
