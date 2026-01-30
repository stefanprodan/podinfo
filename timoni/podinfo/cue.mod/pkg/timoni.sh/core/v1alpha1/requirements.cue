// Copyright 2023 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strconv"
	"strings"
)

// CPUQuantity is a string that is validated as a quantity of CPU, such as 100m or 2000m.
#CPUQuantity: string & =~"^[1-9]\\d*m$"

// MemoryQuantity is a string that is validated as a quantity of memory, such as 128Mi or 2Gi.
#MemoryQuantity: string & =~"^[1-9]\\d*(Mi|Gi)$"

// ResourceRequirement defines the schema for the CPU and Memory resource requirements.
#ResourceRequirement: {
	cpu?:    #CPUQuantity
	memory?: #MemoryQuantity
}

// ResourceRequirements defines the schema for the compute resource requirements of a container.
// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/.
#ResourceRequirements: {
	// Limits describes the maximum amount of compute resources allowed.
	limits?: #ResourceRequirement

	// Requests describes the minimum amount of compute resources required.
	// Requests cannot exceed Limits.
	requests?: #ResourceRequirement & {
		if limits != _|_ {
			if limits.cpu != _|_ {
				_lc:  strconv.Atoi(strings.Split(limits.cpu, "m")[0])
				_rc:  strconv.Atoi(strings.Split(requests.cpu, "m")[0])
				#cpu: int & >=_rc & _lc
			}
		}
	}
}
