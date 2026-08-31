// Copyright 2026 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// AffinityPreset selects how the workload replicas spread across
// topology domains: the default "soft" preset prefers spreading the
// replicas, "hard" requires it and "none" disables the spreading.
#AffinityPreset: *#AffinityPresetSoft | #AffinityPresetHard | #AffinityPresetNone

#AffinityPresetSoft: "soft"
#AffinityPresetHard: "hard"
#AffinityPresetNone: "none"

// AffinityValues defines the canonical module values for a workload's
// `affinity` setting: the podAntiAffinity field accepts an
// #AffinityPreset or raw pod anti-affinity rules, while the node
// affinity and pod affinity rules are always raw. Modules should
// tighten the raw branches with the corev1 schemas, e.g.
// `podAntiAffinity: timoniv1.#AffinityPreset | corev1.#PodAntiAffinity`.
#AffinityValues: {
	// Spread the workload replicas across topology domains: the
	// default "soft" preset prefers spreading, "hard" requires it
	// and "none" disables it; raw pod anti-affinity rules replace
	// the presets entirely.
	podAntiAffinity: #AffinityPreset | {...}

	// Raw node affinity rules (optional); the OS placement should
	// come from the nodeSelector setting instead.
	nodeAffinity?: {...}

	// Raw pod affinity rules (optional).
	podAffinity?: {...}

	...
}

// Affinity generates the pod affinity rules from the #Values input:
// the pod anti-affinity presets render a term matching the
// #MatchLabels pods over the #TopologyKey domain, while raw rules
// pass through unchanged. The #Enabled field tells whether any rules
// are set, letting modules omit the affinity field from the pod spec.
#Affinity: {
	// The affinity values, wired to the module's affinity setting.
	#Values: #AffinityValues

	// The labels selecting the workload's pods, wired to the
	// module's selector labels.
	#MatchLabels: #Labels

	// The topology domain the anti-affinity presets apply to.
	#TopologyKey: *"kubernetes.io/hostname" | string

	// The preference weight of the "soft" anti-affinity preset.
	#Weight: *100 | int & >0 & <=100

	// Whether any affinity rules are set.
	#Enabled: bool
	#Enabled: [
		if #Values.nodeAffinity != _|_ {true},
		if #Values.podAffinity != _|_ {true},
		if (#Values.podAntiAffinity & string) == _|_ {true},
		if (#Values.podAntiAffinity & (#AffinityPresetSoft | #AffinityPresetHard)) != _|_ {true},
		false,
	][0]

	let p = #Values.podAntiAffinity
	if (p & string) == _|_ {
		podAntiAffinity: p
	}
	if (p & string) != _|_ if p == #AffinityPresetSoft {
		podAntiAffinity: preferredDuringSchedulingIgnoredDuringExecution: [{
			weight: #Weight
			podAffinityTerm: {
				labelSelector: matchLabels: #MatchLabels
				topologyKey: #TopologyKey
			}
		}]
	}
	if (p & string) != _|_ if p == #AffinityPresetHard {
		podAntiAffinity: requiredDuringSchedulingIgnoredDuringExecution: [{
			labelSelector: matchLabels: #MatchLabels
			topologyKey: #TopologyKey
		}]
	}
	if #Values.nodeAffinity != _|_ {
		nodeAffinity: #Values.nodeAffinity
	}
	if #Values.podAffinity != _|_ {
		podAffinity: #Values.podAffinity
	}

	...
}
