// Copyright 2023 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Action holds the list of annotations for controlling
// Timoni's apply behaviour of Kubernetes resources.
Action: {
	// Force annotation for recreating immutable resources such as Kubernetes Jobs.
	Force: {
		"action.timoni.sh/force": ActionStatus.Enabled
	}
	// One-off annotation for appling resources only if they don't exist on the cluster.
	Oneoff: {
		"action.timoni.sh/one-off": ActionStatus.Enabled
	}
	// Keep annotation for preventing Timoni's garbage collector from deleting resources.
	Keep: {
		"action.timoni.sh/prune": ActionStatus.Disabled
	}
}

ActionStatus: {
	Enabled:  "enabled"
	Disabled: "disabled"
}
