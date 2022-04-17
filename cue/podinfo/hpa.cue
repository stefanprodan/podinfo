package podinfo

import (
	autoscaling "k8s.io/api/autoscaling/v2beta2"
)

#hpaConfig: {
	enabled:     *false | bool
	cpu:         *99 | int
	memory:      *"" | string
	minReplicas: *1 | int
	maxReplicas: *1 | int
}

#HorizontalPodAutoscaler: autoscaling.#HorizontalPodAutoscaler & {
	_config:    #Config
	apiVersion: "autoscaling/v2beta2"
	kind:       "HorizontalPodAutoscaler"
	metadata:   _config.meta
	spec: {
		scaleTargetRef: {
			apiVersion: "apps/v1"
			kind:       "Deployment"
			name:       _config.meta.name
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
		]
	}
}
