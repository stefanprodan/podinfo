// Copyright 2023 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strings"
)

// Image defines the schema for OCI image reference used in Kubernetes PodSpec container image.
#Image: {

	// Repository is the address of a container registry repository.
	// An image repository is made up of slash-separated name components, optionally
	// prefixed by a registry hostname and port in the format [HOST[:PORT_NUMBER]/]PATH.
	repository!: string

	// Tag identifies an image in the repository.
	// A tag name may contain lowercase and uppercase characters, digits, underscores, periods and dashes.
	// A tag name may not start with a period or a dash and may contain a maximum of 128 characters.
	tag!: string & strings.MaxRunes(128)

	// Digest uniquely and immutably identifies an image in the repository.
	// Spec: https://github.com/opencontainers/image-spec/blob/main/descriptor.md#digests.
	digest!: string

	// PullPolicy defines the pull policy for the image.
	// By default, it is set to IfNotPresent.
	pullPolicy: *"IfNotPresent" | "Always" | "Never"

	// Reference is the image address computed from repository, tag and digest
	// in the format [REPOSITORY]:[TAG]@[DIGEST].
	reference: string

	if digest != "" && tag != "" {
		reference: "\(repository):\(tag)@\(digest)"
	}

	if digest != "" && tag == "" {
		reference: "\(repository)@\(digest)"
	}

	if digest == "" && tag != "" {
		reference: "\(repository):\(tag)"
	}

	if digest == "" && tag == "" {
		reference: "\(repository):latest"
	}
}
