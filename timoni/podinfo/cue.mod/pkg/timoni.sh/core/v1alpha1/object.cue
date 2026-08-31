// Copyright 2023 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "strings"

// ObjectReference is a reference to a Kubernetes object.
#ObjectReference: {
	// Name of the referent.
	name!: string & strings.MaxRunes(256)

	// Namespace of the referent.
	namespace?: string & strings.MaxRunes(256)

	// API version of the referent.
	apiVersion?: string & strings.MaxRunes(256)

	// Kind of the referent.
	kind?: string & strings.MaxRunes(256)
}
