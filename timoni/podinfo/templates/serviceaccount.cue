package templates

import (
	corev1 "k8s.io/api/core/v1"
)

#ServiceAccount: corev1.#ServiceAccount & {
	_config:    #Config
	apiVersion: "v1"
	kind:       "ServiceAccount"
	metadata: {
		name:      _config.serviceAccount.name
		namespace: _config.metadata.namespace
		labels:    _config.metadata.labels
		if _config.serviceAccount.labels != _|_ {
			labels: _config.serviceAccount.labels
		}
		if _config.metadata.annotations != _|_ {
			annotations: _config.metadata.annotations
		}
		if _config.serviceAccount.annotations != _|_ {
			annotations: _config.serviceAccount.annotations
		}
	}
	automountServiceAccountToken: _config.serviceAccount.automountServiceAccountToken
}
