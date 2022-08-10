// Code generated by cue get go. DO NOT EDIT.

//cue:generate cue get go k8s.io/api/apiserverinternal/v1alpha1

package v1alpha1

//  Storage version of a specific resource.
#StorageVersion: {
	// Spec is an empty spec. It is here to comply with Kubernetes API style.
	spec: #StorageVersionSpec @go(Spec) @protobuf(2,bytes,opt)

	// API server instances report the version they can decode and the version they
	// encode objects to when persisting objects in the backend.
	status: #StorageVersionStatus @go(Status) @protobuf(3,bytes,opt)
}

// StorageVersionSpec is an empty spec.
#StorageVersionSpec: {
}

// API server instances report the versions they can decode and the version they
// encode objects to when persisting objects in the backend.
#StorageVersionStatus: {
	// The reported versions per API server instance.
	// +optional
	// +listType=map
	// +listMapKey=apiServerID
	storageVersions?: [...#ServerStorageVersion] @go(StorageVersions,[]ServerStorageVersion) @protobuf(1,bytes,opt)

	// If all API server instances agree on the same encoding storage version,
	// then this field is set to that version. Otherwise this field is left empty.
	// API servers should finish updating its storageVersionStatus entry before
	// serving write operations, so that this field will be in sync with the reality.
	// +optional
	commonEncodingVersion?: null | string @go(CommonEncodingVersion,*string) @protobuf(2,bytes,opt)

	// The latest available observations of the storageVersion's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	conditions?: [...#StorageVersionCondition] @go(Conditions,[]StorageVersionCondition) @protobuf(3,bytes,opt)
}

// An API server instance reports the version it can decode and the version it
// encodes objects to when persisting objects in the backend.
#ServerStorageVersion: {
	// The ID of the reporting API server.
	apiServerID?: string @go(APIServerID) @protobuf(1,bytes,opt)

	// The API server encodes the object to this version when persisting it in
	// the backend (e.g., etcd).
	encodingVersion?: string @go(EncodingVersion) @protobuf(2,bytes,opt)

	// The API server can decode objects encoded in these versions.
	// The encodingVersion must be included in the decodableVersions.
	// +listType=set
	decodableVersions?: [...string] @go(DecodableVersions,[]string) @protobuf(3,bytes,opt)
}

#StorageVersionConditionType: string // #enumStorageVersionConditionType

#enumStorageVersionConditionType:
	#AllEncodingVersionsEqual

// Indicates that encoding storage versions reported by all servers are equal.
#AllEncodingVersionsEqual: #StorageVersionConditionType & "AllEncodingVersionsEqual"

#ConditionStatus: string // #enumConditionStatus

#enumConditionStatus:
	#ConditionTrue |
	#ConditionFalse |
	#ConditionUnknown

#ConditionTrue:    #ConditionStatus & "True"
#ConditionFalse:   #ConditionStatus & "False"
#ConditionUnknown: #ConditionStatus & "Unknown"

// Describes the state of the storageVersion at a certain point.
#StorageVersionCondition: {
	// Type of the condition.
	// +required
	type: #StorageVersionConditionType @go(Type) @protobuf(1,bytes,opt)

	// Status of the condition, one of True, False, Unknown.
	// +required
	status: #ConditionStatus @go(Status) @protobuf(2,bytes,opt)

	// If set, this represents the .metadata.generation that the condition was set based upon.
	// +optional
	observedGeneration?: int64 @go(ObservedGeneration) @protobuf(3,varint,opt)

	// The reason for the condition's last transition.
	// +required
	reason: string @go(Reason) @protobuf(5,bytes,opt)

	// A human readable message indicating details about the transition.
	// +required
	message?: string @go(Message) @protobuf(6,bytes,opt)
}

// A list of StorageVersions.
#StorageVersionList: {
	// Items holds a list of StorageVersion
	items: [...#StorageVersion] @go(Items,[]StorageVersion) @protobuf(2,bytes,rep)
}
