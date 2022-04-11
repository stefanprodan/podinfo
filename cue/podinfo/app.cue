package podinfo

#Application: {
	input: #Config
	out: {
		sa:     #ServiceAccount & {_config: input}
		deploy: #Deployment & {
			_config:         input
			_serviceAccount: sa.metadata.name
		}
		service: #Service & {_config: input}}
	if input.hpa.enabled == true {
		out: hpa: #HorizontalPodAutoscaler & {_config: input}
	}
	if input.serviceMonitor.enabled == true {
		out: serviceMonitor: #ServiceMonitor & {_config: input}
	}
	if input.ingress.enabled == true {
		out: ingress: #Ingress & {_config: input}
	}
}
