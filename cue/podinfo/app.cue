package podinfo

#Application: {
	config: #Config

	objects: {
		service:    #Service & {_config:        config}
		account:    #ServiceAccount & {_config: config}
		deployment: #Deployment & {
			_config:         config
			_serviceAccount: account.metadata.name
		}
	}

	if config.hpa.enabled == true {
		objects: hpa: #HorizontalPodAutoscaler & {_config: config}
	}

	if config.ingress.enabled == true {
		objects: ingress: #Ingress & {_config: config}
	}

	if config.serviceMonitor.enabled == true {
		objects: serviceMonitor: #ServiceMonitor & {_config: config}
	}
}
