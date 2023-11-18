// Copyright 2023 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "strings"

// Selector defines the schema for Kubernetes Pod label selector used in Deployments, Services, Jobs, etc.
#Selector: {
	// Name must be unique within a namespace. Is required when creating resources.
	// Name is primarily intended for creation idempotence and configuration definition.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#names
	#Name!: string & =~"^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$" & strings.MinRunes(1) & strings.MaxRunes(63)

	// Map of string keys and values that can be used to organize and categorize (scope and select) objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	labels: {[string & =~"^(([A-Za-z0-9][-A-Za-z0-9_./]*)?[A-Za-z0-9])?$" & strings.MaxRunes(63)]: string & =~"^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$" & strings.MaxRunes(63)}

	// Standard Kubernetes label: app name.
	labels: "app.kubernetes.io/name": #Name
}
