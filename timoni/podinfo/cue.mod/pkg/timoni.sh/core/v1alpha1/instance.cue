// Copyright 2023 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "strings"

// InstanceName defines the schema for the name of a Timoni instance.
// The instance name is used to name Kubernetes resources and as a label value,
// so it must be lowercase and 63 characters or less.
#InstanceName: string & =~"^(([a-z0-9][-a-z0-9_.]*)?[a-z0-9])?$" & strings.MinRunes(1) & strings.MaxRunes(63)

// InstanceNamespace defines the schema for the namespace of a Timoni instance.
// The instance namespace is used as a Kubernetes namespace and label value,
// so it must be lowercase and 63 characters or less.
#InstanceNamespace: string & =~"^(([a-z0-9][-a-z0-9_.]*)?[a-z0-9])?$" & strings.MinRunes(1) & strings.MaxRunes(63)

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
