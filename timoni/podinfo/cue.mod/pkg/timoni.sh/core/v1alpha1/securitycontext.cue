// Copyright 2026 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// SecurityContextPreset selects how the workload identity defaults are
// applied. The "hardened" preset pins the numeric UID/GID defaults; the
// "platform" preset omits them so that an admission controller (e.g. an
// OpenShift SecurityContextConstraint) can assign them from the range
// allocated to the namespace.
#SecurityContextPreset: *#SecurityContextPresetHardened | #SecurityContextPresetPlatform

#SecurityContextPresetHardened: "hardened"
#SecurityContextPresetPlatform: "platform"

// ContainerSecurityContext is the restricted security context module
// containers should default to. It denies privilege escalation, makes
// the root filesystem read-only and drops all capabilities. Identity
// (UID/GID) and the seccomp profile are pod-level concerns, see
// #PodSecurityContext. All fields remain overridable and the struct
// stays open for unification with the corev1.#SecurityContext schema.
#ContainerSecurityContext: {
	allowPrivilegeEscalation: *false | bool
	readOnlyRootFilesystem:   *true | bool
	capabilities: drop: *["ALL"] | [...string]
	...
}

// PodSecurityContext generates the pod-level security context for the
// given #SecurityContextPreset. The runAsNonRoot and RuntimeDefault
// seccomp profile defaults satisfy the Kubernetes restricted Pod
// Security Standard under both presets; the numeric identity defaults
// (runAsUser, runAsGroup, fsGroup) are added only under the "hardened"
// preset and only when the module declares its image's non-root UID
// via #User. The struct stays open for unification with the
// corev1.#PodSecurityContext schema.
#PodSecurityContext: {
	// The preset, wired to the module's securityContextPreset value.
	#Preset: #SecurityContextPreset

	// The non-root UID the container image runs as. Modules whose
	// upstream pins no identity leave it unset, which makes the
	// preset a no-op (identity fields are never rendered).
	#User?: int

	// Optional overrides when the group or volume group differs from
	// #User (e.g. an image pairing UID 65532 with fsGroup 65534).
	#Group?:   int
	#FSGroup?: int

	runAsNonRoot: *true | bool
	seccompProfile: type: *"RuntimeDefault" | string
	if #Preset == #SecurityContextPresetHardened if #User != _|_ {
		runAsUser: *#User | int
		runAsGroup: *[if #Group != _|_ {#Group}, #User][0] | int
		fsGroup: *[if #FSGroup != _|_ {#FSGroup}, #User][0] | int
	}
	...
}
