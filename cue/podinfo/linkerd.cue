package podinfo

#ServiceProfile: {
	_config:    #Config
	apiVersion: "v1alpha2"
	kind:       "ServiceProfile"
	metadata:   _config.meta
	spec: {
		routes: [ for r in routes {
			condition: {
				method:    r.method
				pathRegex: r.path
			}
		}]
	}
}

routes: [
	{
		method: "GET"
		path:   "/"
	},
]
