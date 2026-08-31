// Copyright 2026 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "strings"

// Bundle defines the schema for a Timoni bundle that describes
// a set of instances to be applied to a cluster.
#Bundle: {
	apiVersion: string & =~"^v1alpha1$"
	name:       string & =~"^(([a-z0-9][-a-z0-9_.]*)?[a-z0-9])?$" & strings.MaxRunes(63) & strings.MinRunes(1)
	instances: [string & =~"^(([a-z0-9][-a-z0-9_.]*)?[a-z0-9])?$" & strings.MaxRunes(63) & strings.MinRunes(1)]: {
		module: close({
			url:     string & =~"^(oci|file)://.*$"
			version: *"latest" | string
			digest?: string
		})
		namespace: string & =~"^(([a-z0-9][-a-z0-9_.]*)?[a-z0-9])?$" & strings.MaxRunes(63) & strings.MinRunes(1)
		values: {...}
	}
}
