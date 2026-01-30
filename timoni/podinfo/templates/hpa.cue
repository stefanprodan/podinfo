package templates

import (
	autoscaling "k8s.io/api/autoscaling/v2"
)

#HorizontalPodAutoscaler: autoscaling.#HorizontalPodAutoscaler & {
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
		minReplicas: _config.autoscaling.minReplicas
		maxReplicas: _config.autoscaling.maxReplicas
		metrics: [
			if _config.autoscaling.cpu > 0 {
				{
					type: "Resource"
					resource: {
						name: "cpu"
						target: {
							type:               "Utilization"
							averageUtilization: _config.autoscaling.cpu
						}
					}
				}
			},
			if _config.autoscaling.memory != "" {
				{
					type: "Resource"
					resource: {
						name: "memory"
						target: {
							type:         "AverageValue"
							averageValue: _config.autoscaling.memory
						}
					}
				}
			},
		]
	}
}
