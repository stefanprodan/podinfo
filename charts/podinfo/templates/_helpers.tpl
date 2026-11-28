{{/*
Expand the name of the chart.
*/}}
{{- define "podinfo.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "podinfo.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Allow the release namespace to be overridden for multi-namespace deployments in combined charts.
*/}}
{{- define "podinfo.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "podinfo.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "podinfo.labels" -}}
helm.sh/chart: {{ include "podinfo.chart" . }}
{{ include "podinfo.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "podinfo.selectorLabels" -}}
app.kubernetes.io/name: {{ include "podinfo.fullname" . }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "podinfo.serviceAccountName" -}}
{{- if .Values.serviceAccount.enabled }}
{{- default (include "podinfo.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the TLS secret for the secure port.
*/}}
{{- define "podinfo.tlsSecretName" -}}
{{- $fullname := include "podinfo.fullname" . -}}
{{- default (printf "%s-tls" $fullname) .Values.tls.secretName }}
{{- end }}

{{/*
True when podinfo caches to the bundled Redis over mutual TLS.
*/}}
{{- define "podinfo.redisTLS" -}}
{{- if and .Values.redis.enabled .Values.redis.tls.enabled }}true{{ end }}
{{- end }}

{{/*
The directory of the SPIFFE Workload API socket, where the CSI volume is mounted.
*/}}
{{- define "podinfo.spiffeSocketDir" -}}
{{- dir (trimPrefix "unix://" .Values.spiffe.endpointSocket) }}
{{- end }}

{{/*
The OpenTelemetry environment variables, except the ones set in extraEnvs.
The Kubernetes resource attributes are set from the downward API.
*/}}
{{- define "podinfo.otelEnv" -}}
{{- $fullname := include "podinfo.fullname" . }}
{{- $namespace := include "podinfo.namespace" . }}
{{- $attributes := dict
  "k8s.namespace.name" $namespace
  "k8s.deployment.name" $fullname
  "k8s.container.name" .Chart.Name
  "k8s.pod.name" "$(OTEL_RESOURCE_ATTRIBUTES_POD_NAME)"
  "k8s.pod.uid" "$(OTEL_RESOURCE_ATTRIBUTES_POD_UID)"
  "k8s.node.name" "$(OTEL_RESOURCE_ATTRIBUTES_NODE_NAME)"
  "service.namespace" $namespace
}}
{{- $pairs := list }}
{{- range $key, $value := mergeOverwrite $attributes .Values.otel.resourceAttributes }}
{{- $pairs = append $pairs (printf "%s=%v" $key $value) }}
{{- end }}
{{- $env := list
  (dict "name" "OTEL_SERVICE_NAME" "value" (default $fullname .Values.otel.serviceName))
  (dict "name" "OTEL_EXPORTER_OTLP_ENDPOINT" "value" (required "otel.endpoint is required when otel.enabled" .Values.otel.endpoint))
}}
{{- with .Values.otel.protocol }}
{{- $env = append $env (dict "name" "OTEL_EXPORTER_OTLP_PROTOCOL" "value" .) }}
{{- end }}
{{- with .Values.otel.sampler }}
{{- $env = append $env (dict "name" "OTEL_TRACES_SAMPLER" "value" .) }}
{{- end }}
{{- with .Values.otel.samplerArg }}
{{- $env = append $env (dict "name" "OTEL_TRACES_SAMPLER_ARG" "value" (toString .)) }}
{{- end }}
{{- $env = concat $env (list
  (dict "name" "OTEL_RESOURCE_ATTRIBUTES_POD_NAME" "valueFrom" (dict "fieldRef" (dict "fieldPath" "metadata.name")))
  (dict "name" "OTEL_RESOURCE_ATTRIBUTES_POD_UID" "valueFrom" (dict "fieldRef" (dict "fieldPath" "metadata.uid")))
  (dict "name" "OTEL_RESOURCE_ATTRIBUTES_NODE_NAME" "valueFrom" (dict "fieldRef" (dict "fieldPath" "spec.nodeName")))
  (dict "name" "OTEL_RESOURCE_ATTRIBUTES" "value" (join "," $pairs))
) }}
{{- $overrides := list }}
{{- range .Values.extraEnvs }}
{{- $overrides = append $overrides .name }}
{{- end }}
{{- $out := list }}
{{- range $env }}
{{- if not (has .name $overrides) }}
{{- $out = append $out . }}
{{- end }}
{{- end }}
{{- with $out }}
{{- toYaml . }}
{{- end }}
{{- end }}
