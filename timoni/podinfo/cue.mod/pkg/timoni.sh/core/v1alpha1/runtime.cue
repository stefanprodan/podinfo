// Copyright 2026 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "strings"

// RuntimeValue defines the schema for a Timoni runtime value
// fetched from the in-cluster Kubernetes resources.
#RuntimeValue: {
	query: string
	for: {[string & =~"^(([A-Za-z0-9][-A-Za-z0-9_]*)?[A-Za-z0-9])?$" & strings.MaxRunes(63) & strings.MinRunes(1)]: string}
	optional: *false | bool
}

// Runtime defines the schema for a Timoni runtime that describes
// the target clusters and the values fetched from their resources.
#Runtime: {
	apiVersion: string & =~"^v1alpha1$"
	name:       string & =~"^(([a-z0-9][-a-z0-9_]*)?[a-z0-9])?$" & strings.MaxRunes(63) & strings.MinRunes(1)

	clusters?: [string & =~"^(([a-z0-9][-a-z0-9_.]*)?[a-z0-9])?$" & strings.MaxRunes(63) & strings.MinRunes(1)]: {
		group!:       string
		kubeContext!: string
	}

	values?: [...#RuntimeValue]
}
