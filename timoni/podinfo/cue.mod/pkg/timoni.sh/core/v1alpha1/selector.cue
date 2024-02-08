// Copyright 2023 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Selector defines the schema for Kubernetes Pod label selector used in Deployments, Services, Jobs, etc.
#Selector: {
	// Name must be unique within a namespace. Is required when creating resources.
	// Name is primarily intended for creation idempotence and configuration definition.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#names
	#Name!: #InstanceName

	// Map of string keys and values that can be used to organize and categorize (scope and select) objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	labels: #Labels

	// Standard Kubernetes label: app name.
	labels: "\(#StdLabelName)": #Name
}
