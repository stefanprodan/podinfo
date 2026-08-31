// Copyright 2026 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// #HealthCheck defines a custom readiness evaluation for Kubernetes
// custom resources that are not compliant with kstatus conventions.
// While waiting for resources to become ready, Timoni fills #object
// with the live object read from the cluster and evaluates the boolean
// expressions in order: inProgress, failed, current. The first
// expression that evaluates to true decides the resource status; when
// none does, the resource counts as in progress and is polled until
// the timeout expires. An expression that cannot be evaluated because
// the referenced fields are missing from the live object counts as
// false.
#HealthCheck: {
	// Group of the target resources e.g. 'cert-manager.io', applying
	// to every version of the group. Health checks are meant for
	// custom resources; the built-in kinds are covered by kstatus.
	group!: string & =~"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"

	// Kind of the target resource e.g. 'Certificate'. When omitted,
	// the health check applies to every kind in the group.
	kind?: string

	// #object is filled by Timoni with the live Kubernetes object.
	// Constrain it here to type the expressions below.
	#object: {...}

	// current must evaluate to true when the resource is ready.
	current!: bool

	// failed should evaluate to true when the resource has reached a
	// terminal failure state; it aborts the wait early.
	failed?: bool

	// inProgress should evaluate to true when the resource is known
	// to be reconciling e.g. when the reported status is stale.
	inProgress?: bool

	...
}

// #HealthCheckForCondition generates a #HealthCheck for resources that
// signal readiness through status conditions, the common CRD pattern:
// ready when the <conditionType> condition is True and the reported
// status is fresh; failed when the affirmative <failedConditionType>
// condition is True and fresh. Generation staleness is checked
// wherever the resource tracks it, and skipped where it doesn't:
//   - a top-level status.observedGeneration lagging metadata.generation
//     reports in progress;
//   - a condition carrying its own observedGeneration only counts as
//     ready or failed when it matches metadata.generation;
//   - resources with neither simply gate on the condition status;
//   - resources without a <failedConditionType> condition never fail
//     fast.
#HealthCheckForCondition: #HealthCheck & {
	// Type of the status condition that signals readiness.
	conditionType: string | *"Ready"

	// Type of the status condition that signals terminal failure.
	failedConditionType: string | *"Stalled"

	#object: {
		metadata: {generation?: int, ...}
		status?: {
			observedGeneration?: int
			conditions?: [...{type?: string, status?: string, observedGeneration?: int, ...}]
			...
		}
		...
	}

	inProgress: [
		if #object.status.observedGeneration != _|_ {
			#object.status.observedGeneration != #object.metadata.generation
		},
		false,
	][0]

	current: len([
		for c in #object.status.conditions
		if c.type == conditionType && c.status == "True"
		if [
			if c.observedGeneration != _|_ {c.observedGeneration == #object.metadata.generation},
			true,
		][0] {c},
	]) > 0

	failed: len([
		for c in #object.status.conditions
		if c.type == failedConditionType && c.status == "True"
		if [
			if c.observedGeneration != _|_ {c.observedGeneration == #object.metadata.generation},
			true,
		][0] {c},
	]) > 0
}
