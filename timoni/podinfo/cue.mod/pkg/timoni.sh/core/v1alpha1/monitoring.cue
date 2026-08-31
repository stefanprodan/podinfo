// Copyright 2026 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// PromDuration is a duration in Prometheus format,
// e.g. "30s" or "1m30s"; a bare "0" is allowed.
#PromDuration: !="" & =~"^(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$"

// Monitor defines the module values shared by the Prometheus Operator
// ServiceMonitor and PodMonitor resources. Modules scraping a single
// endpoint should expose #MonitorValues; modules scraping several
// endpoints combine #Monitor with one #MonitorEndpoint per endpoint.
#Monitor: {
	// Create the monitor; requires the Prometheus Operator CRDs.
	enabled: *false | bool

	// Extra labels added to the monitor metadata, e.g. for matching
	// a Prometheus instance monitor selector.
	additionalLabels?: #Labels

	// Extra annotations added to the monitor metadata.
	annotations?: #Annotations

	// The label to use to retrieve the Prometheus job name from.
	jobLabel: *#StdLabelName | string

	// Per-scrape limits; unset fields fall back to the
	// Prometheus defaults.
	sampleLimit?:           int & >=0
	targetLimit?:           int & >=0
	labelLimit?:            int & >=0
	labelNameLengthLimit?:  int & >=0
	labelValueLengthLimit?: int & >=0

	// Service labels transferred onto the metrics
	// (ServiceMonitor only).
	targetLabels?: [...string & =~".+"]

	// Pod labels transferred onto the metrics.
	podTargetLabels?: [...string & =~".+"]

	...
}

// MonitorEndpoint defines the module values for one monitor scrape
// endpoint. Duration fields set to the default empty string are
// omitted from the rendered endpoint and fall back to the Prometheus
// defaults: the Prometheus administrator owns the scrape cadence.
#MonitorEndpoint: {
	// The scrape interval; empty defers to the Prometheus default.
	interval: *"" | #PromDuration

	// The scrape request timeout; empty defers to the Prometheus default.
	scrapeTimeout: *"" | #PromDuration

	// Keep the metrics labels on collision with the target labels.
	honorLabels: *false | bool

	// The scheme of the metrics endpoint.
	scheme?: "http" | "https"

	// Whether to enable HTTP2 for scraping.
	enableHttp2?: bool

	// The TLS settings of the scrape requests,
	// e.g. `{insecureSkipVerify: true}`.
	tlsConfig?: {...}

	// The file the scrape bearer token is read from.
	bearerTokenFile?: string & =~".+"

	// The Kubernetes Secret key the scrape bearer token is read from.
	bearerTokenSecret?: {...}

	// The proxy URL the scrape requests go through.
	proxyUrl?: string & =~".+"

	// Relabeling rules applied to the samples before ingestion.
	metricRelabelings?: [...]

	// Relabeling rules applied to the target before scraping.
	relabelings?: [...]

	...
}

// MonitorValues defines the canonical module values for a
// `serviceMonitor` or `podMonitor` setting scraping a single endpoint.
#MonitorValues: #Monitor & #MonitorEndpoint

// MonitorSpec generates the ServiceMonitor/PodMonitor spec fields
// common to both kinds from the #Values input, omitting the unset
// limits and target labels. The selector, namespace selector and
// endpoints remain module concerns. Note that the PodMonitor kind
// rejects `targetLabels` at validation time, being a Service concept.
#MonitorSpec: {
	// The monitor values, wired to the module's
	// serviceMonitor or podMonitor setting.
	#Values: #Monitor

	jobLabel: #Values.jobLabel
	if #Values.sampleLimit != _|_ {
		sampleLimit: #Values.sampleLimit
	}
	if #Values.targetLimit != _|_ {
		targetLimit: #Values.targetLimit
	}
	if #Values.labelLimit != _|_ {
		labelLimit: #Values.labelLimit
	}
	if #Values.labelNameLengthLimit != _|_ {
		labelNameLengthLimit: #Values.labelNameLengthLimit
	}
	if #Values.labelValueLengthLimit != _|_ {
		labelValueLengthLimit: #Values.labelValueLengthLimit
	}
	if #Values.targetLabels != _|_ {
		targetLabels: #Values.targetLabels
	}
	if #Values.podTargetLabels != _|_ {
		podTargetLabels: #Values.podTargetLabels
	}

	...
}

// MonitorEndpointSpec generates a ServiceMonitor endpoint or a
// PodMonitor podMetricsEndpoint from the #Values input, omitting the
// unset scrape settings. The port, path and other endpoint
// identifiers remain module concerns.
#MonitorEndpointSpec: {
	// The endpoint values, wired to the module's
	// serviceMonitor or podMonitor setting.
	#Values: #MonitorEndpoint

	honorLabels: #Values.honorLabels
	if #Values.interval != "" {
		interval: #Values.interval
	}
	if #Values.scrapeTimeout != "" {
		scrapeTimeout: #Values.scrapeTimeout
	}
	if #Values.scheme != _|_ {
		scheme: #Values.scheme
	}
	if #Values.enableHttp2 != _|_ {
		enableHttp2: #Values.enableHttp2
	}
	if #Values.tlsConfig != _|_ {
		tlsConfig: #Values.tlsConfig
	}
	if #Values.bearerTokenFile != _|_ {
		bearerTokenFile: #Values.bearerTokenFile
	}
	if #Values.bearerTokenSecret != _|_ {
		bearerTokenSecret: #Values.bearerTokenSecret
	}
	if #Values.proxyUrl != _|_ {
		proxyUrl: #Values.proxyUrl
	}
	if #Values.metricRelabelings != _|_ {
		metricRelabelings: #Values.metricRelabelings
	}
	if #Values.relabelings != _|_ {
		relabelings: #Values.relabelings
	}

	...
}
