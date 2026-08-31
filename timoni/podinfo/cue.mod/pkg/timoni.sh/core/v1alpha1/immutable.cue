// Copyright 2024 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"strings"
	"uuid"
)

#ConfigMapKind: "ConfigMap"
#SecretKind:    "Secret"

// ImmutableConfig is a generator for immutable Kubernetes ConfigMaps and Secrets.
// The metadata.name of the generated object is suffixed with the hash of the input data.
#ImmutableConfig: {
	// Kind of the generated object.
	#Kind: *#ConfigMapKind | #SecretKind

	// Metadata of the generated object. Accepts both #Metadata and
	// #MetaComponent values: the object name may carry a component
	// suffix while the name label keeps the instance name.
	#Meta: {
		name!:        string
		namespace!:   string
		labels:       #Labels
		annotations?: #Annotations
		...
	}

	// Optional suffix appended to the generate name.
	#Suffix: *"" | string

	// Data of the generated object.
	#Data: {[string]: string}

	let hash = strings.Split(uuid.SHA1(uuid.ns.DNS, json.Marshal(#Data)), "-")[0]

	apiVersion: "v1"
	kind:       #Kind
	metadata: {
		name:      #Meta.name + #Suffix + "-" + hash
		namespace: #Meta.namespace
		labels:    #Meta.labels
		if #Meta.annotations != _|_ {
			annotations: #Meta.annotations
		}
	}
	immutable: true
	if kind == #ConfigMapKind {
		data: #Data
	}
	if kind == #SecretKind {
		stringData: #Data
	}
}
