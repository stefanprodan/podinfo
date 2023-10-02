// Copyright 2023 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "strings"

// Metadata defines the schema for Kubernetes object metadata.
#Metadata: {
	// Version should be in the strict semver format. Is required when creating resources.
	#Version!: string & strings.MaxRunes(63)

	// Name must be unique within a namespace. Is required when creating resources.
	// Name is primarily intended for creation idempotence and configuration definition.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#names
	name!: string & =~"^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$" & strings.MaxRunes(63)

	// Namespace defines the space within which each name must be unique.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces
	namespace!: string & =~"^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$" & strings.MaxRunes(63)

	// Annotations is an unstructured key value map stored with a resource that may be
	// set to store and retrieve arbitrary metadata.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations
	annotations?: {[string & =~"^(([A-Za-z0-9][-A-Za-z0-9_./]*)?[A-Za-z0-9])?$" & strings.MaxRunes(63)]: string}

	// Map of string keys and values that can be used to organize and categorize (scope and select) objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	labels: {[string & =~"^(([A-Za-z0-9][-A-Za-z0-9_./]*)?[A-Za-z0-9])?$" & strings.MaxRunes(63)]: string & =~"^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$" & strings.MaxRunes(63)}

	// Standard Kubernetes labels: app name and version.
	labels: {
		"app.kubernetes.io/name":    name
		"app.kubernetes.io/version": #Version
	}
}
