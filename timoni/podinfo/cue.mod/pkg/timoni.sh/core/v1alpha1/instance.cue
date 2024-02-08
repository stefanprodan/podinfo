// Copyright 2023 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "strings"

// InstanceName defines the schema for the name of a Timoni instance.
// The instance name is used as a Kubernetes label value and must be 63 characters or less.
#InstanceName: string & =~"^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$" & strings.MinRunes(1) & strings.MaxRunes(63)

// InstanceNamespace defines the schema for the namespace of a Timoni instance.
// The instance namespace is used as a Kubernetes label value and must be 63 characters or less.
#InstanceNamespace: string & =~"^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$" & strings.MinRunes(1) & strings.MaxRunes(63)

// InstanceOwnerReference defines the schema for Kubernetes labels used to denote ownership.
#InstanceOwnerReference: {
	#Name:      "instance.timoni.sh/name"
	#Namespace: "instance.timoni.sh/namespace"
}

// InstanceModule defines the schema for the Module of a Timoni instance.
#InstanceModule: {
	url:     string & =~"^((oci|file)://.*)$"
	version: *"latest" | string
	digest?: string
}
