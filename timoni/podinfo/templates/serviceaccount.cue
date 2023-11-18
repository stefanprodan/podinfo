package templates

import (
	corev1 "k8s.io/api/core/v1"
)

#ServiceAccount: corev1.#ServiceAccount & {
	_config:    #Config
	apiVersion: "v1"
	kind:       "ServiceAccount"
	metadata:   _config.metadata
}
