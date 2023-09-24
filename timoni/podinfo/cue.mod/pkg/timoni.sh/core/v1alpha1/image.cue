package v1alpha1

// Image defines the schema for an OCI image reference.
#Image: {
	repository!: string
	tag!:        string
	digest!:     string

	// Reference is the image address computed from
	// repository, tag and digest.
	reference: string

	if digest != "" {
		reference: "\(repository):\(tag)@\(digest)"
	}
	if digest == "" {
		reference: "\(repository):\(tag)"
	}
}
