// Code generated by timoni. DO NOT EDIT.

//timoni:generate timoni import crd -f https://github.com/prometheus-operator/prometheus-operator/releases/download/v0.68.0/stripped-down-crds.yaml

package v1

import "strings"

#ServiceMonitor: {
	apiVersion: "monitoring.coreos.com/v1"
	kind:       "ServiceMonitor"
	metadata!: {
		name!: strings.MaxRunes(253) & strings.MinRunes(1) & {
			string
		}
		namespace!: strings.MaxRunes(63) & strings.MinRunes(1) & {
			string
		}
		labels?: {
			[string]: string
		}
		annotations?: {
			[string]: string
		}
	}
	spec!: #ServiceMonitorSpec
}
#ServiceMonitorSpec: {
	attachMetadata?: {
		node?: bool
	}
	endpoints: [...{
		authorization?: {
			credentials?: {
				key:       string
				name?:     string
				optional?: bool
			}
			type?: string
		}
		basicAuth?: {
			password?: {
				key:       string
				name?:     string
				optional?: bool
			}
			username?: {
				key:       string
				name?:     string
				optional?: bool
			}
		}
		bearerTokenFile?: string
		bearerTokenSecret?: {
			key:       string
			name?:     string
			optional?: bool
		}
		enableHttp2?:     bool
		filterRunning?:   bool
		followRedirects?: bool
		honorLabels?:     bool
		honorTimestamps?: bool
		interval?:        =~"^(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$"
		metricRelabelings?: [...{
			action?:      "replace" | "Replace" | "keep" | "Keep" | "drop" | "Drop" | "hashmod" | "HashMod" | "labelmap" | "LabelMap" | "labeldrop" | "LabelDrop" | "labelkeep" | "LabelKeep" | "lowercase" | "Lowercase" | "uppercase" | "Uppercase" | "keepequal" | "KeepEqual" | "dropequal" | "DropEqual" | *"replace"
			modulus?:     int
			regex?:       string
			replacement?: string
			separator?:   string
			sourceLabels?: [...=~"^[a-zA-Z_][a-zA-Z0-9_]*$"]
			targetLabel?: string
		}]
		oauth2?: {
			clientId: {
				configMap?: {
					key:       string
					name?:     string
					optional?: bool
				}
				secret?: {
					key:       string
					name?:     string
					optional?: bool
				}
			}
			clientSecret: {
				key:       string
				name?:     string
				optional?: bool
			}
			endpointParams?: {
				[string]: string
			}
			scopes?: [...string]
			tokenUrl: strings.MinRunes(1)
		}
		params?: {
			[string]: [...string]
		}
		path?:     string
		port?:     string
		proxyUrl?: string
		relabelings?: [...{
			action?:      "replace" | "Replace" | "keep" | "Keep" | "drop" | "Drop" | "hashmod" | "HashMod" | "labelmap" | "LabelMap" | "labeldrop" | "LabelDrop" | "labelkeep" | "LabelKeep" | "lowercase" | "Lowercase" | "uppercase" | "Uppercase" | "keepequal" | "KeepEqual" | "dropequal" | "DropEqual" | *"replace"
			modulus?:     int
			regex?:       string
			replacement?: string
			separator?:   string
			sourceLabels?: [...=~"^[a-zA-Z_][a-zA-Z0-9_]*$"]
			targetLabel?: string
		}]
		scheme?:        "http" | "https"
		scrapeTimeout?: =~"^(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$"
		targetPort?:    (int | string) & {
			string
		}
		tlsConfig?: {
			ca?: {
				configMap?: {
					key:       string
					name?:     string
					optional?: bool
				}
				secret?: {
					key:       string
					name?:     string
					optional?: bool
				}
			}
			caFile?: string
			cert?: {
				configMap?: {
					key:       string
					name?:     string
					optional?: bool
				}
				secret?: {
					key:       string
					name?:     string
					optional?: bool
				}
			}
			certFile?:           string
			insecureSkipVerify?: bool
			keyFile?:            string
			keySecret?: {
				key:       string
				name?:     string
				optional?: bool
			}
			serverName?: string
		}
	}]
	jobLabel?:              string
	keepDroppedTargets?:    int
	labelLimit?:            int
	labelNameLengthLimit?:  int
	labelValueLengthLimit?: int
	namespaceSelector?: {
		any?: bool
		matchNames?: [...string]
	}
	podTargetLabels?: [...string]
	sampleLimit?: int
	selector: {
		matchExpressions?: [...{
			key:      string
			operator: string
			values?: [...string]
		}]
		matchLabels?: {
			[string]: string
		}
	}
	targetLabels?: [...string]
	targetLimit?: int
}
