// Copyright 2026 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Timoni defines the schema for a module's instance, holding the
// instance configuration and the Kubernetes resources to apply.
#Timoni: {
	apiVersion: string & =~"^v1alpha1$"
	instance: {...}
	apply: [string]: [...]
	healthChecks?: [string]: #HealthCheck
	kubeMinorVersion?: int
}
