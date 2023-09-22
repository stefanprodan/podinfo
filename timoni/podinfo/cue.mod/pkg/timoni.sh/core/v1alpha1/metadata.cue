package v1alpha1

import "strings"

// Metadata defines the schema for the Kubernetes object metadata.
#Metadata: {
	// Name must be unique within a namespace. Is required when creating resources.
	// Name is primarily intended for creation idempotence and configuration definition.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#names
	name!: string & =~"^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$" & strings.MaxRunes(63)

	// Namespace defines the space within which each name must be unique.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces
	namespace!: string & =~"^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$" & strings.MaxRunes(63)

	// Version should be in the strict semver format. Is required when creating resources.
	version!: string & strings.MaxRunes(63)

	// Annotations is an unstructured key value map stored with a resource that may be
	// set o store and retrieve arbitrary metadata.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations
	annotations?: {[string]: string}

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	labels: {[string]: string & =~"^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$" & strings.MaxRunes(63)}

	// Standard Kubernetes labels: app name and version.
	labels: {
		"app.kubernetes.io/name":    name
		"app.kubernetes.io/version": version
	}

	// Labels used to select pods for Kubernetes Deployment, Service, Job, etc.
	labelSelector: *{
			"app.kubernetes.io/name": name
	} | {[ string]: string}
}
