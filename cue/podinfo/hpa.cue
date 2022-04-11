package podinfo

import (
	autoscaling "k8s.io/api/autoscaling/v2beta2"
)

#hpaConfig: {
	enabled:     *false | bool
	cpu:         int
	memory:      string
	minReplicas: int
	maxReplicas: int
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
		metrics: [ {
			type: "Resource"
			resource: {
				name: "cpu"
				target: {
					type:               "Utilization"
					averageUtilization: _config.hpa.cpu
				}
			}
		}, {
			type: "Resource"
			resource: {
				name: "memory"
				target: {
					type:         "AverageValue"
					averageValue: _config.hpa.memory
				}
			}
		}]
	}
}
