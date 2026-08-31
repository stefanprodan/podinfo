package templates

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

#HorizontalPodAutoscaler: autoscalingv2.#HorizontalPodAutoscaler & {
	_config:    #Config
	apiVersion: "autoscaling/v2"
	kind:       "HorizontalPodAutoscaler"
	metadata:   _config.metadata
	spec: {
		scaleTargetRef: {
			apiVersion: "apps/v1"
			kind:       "Deployment"
			name:       _config.metadata.name
		}
		minReplicas: _config.hpa.minReplicas
		maxReplicas: _config.hpa.maxReplicas
		metrics: [
			if _config.hpa.cpu > 0 {
				{
					type: "Resource"
					resource: {
						name: "cpu"
						target: {
							type:               "Utilization"
							averageUtilization: _config.hpa.cpu
						}
					}
				}
			},
			if _config.hpa.memory != "" {
				{
					type: "Resource"
					resource: {
						name: "memory"
						target: {
							type:         "AverageValue"
							averageValue: _config.hpa.memory
						}
					}
				}
			},
			for m in _config.hpa.metrics {m},
		]
		if _config.hpa.behavior != _|_ {
			behavior: _config.hpa.behavior
		}
	}
}
