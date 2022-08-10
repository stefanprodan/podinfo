// Code generated by cue get go. DO NOT EDIT.

//cue:generate cue get go k8s.io/api/authentication/v1

package v1

// ImpersonateUserHeader is used to impersonate a particular user during an API server request
#ImpersonateUserHeader: "Impersonate-User"

// ImpersonateGroupHeader is used to impersonate a particular group during an API server request.
// It can be repeated multiplied times for multiple groups.
#ImpersonateGroupHeader: "Impersonate-Group"

// ImpersonateUIDHeader is used to impersonate a particular UID during an API server request
#ImpersonateUIDHeader: "Impersonate-Uid"

// ImpersonateUserExtraHeaderPrefix is a prefix for any header used to impersonate an entry in the
// extra map[string][]string for user.Info.  The key will be every after the prefix.
// It can be repeated multiplied times for multiple map keys and the same key can be repeated multiple
// times to have multiple elements in the slice under a single key
#ImpersonateUserExtraHeaderPrefix: "Impersonate-Extra-"

// TokenReview attempts to authenticate a token to a known user.
// Note: TokenReview requests may be cached by the webhook token authenticator
// plugin in the kube-apiserver.
#TokenReview: {
	// Spec holds information about the request being evaluated
	spec: #TokenReviewSpec @go(Spec) @protobuf(2,bytes,opt)

	// Status is filled in by the server and indicates whether the request can be authenticated.
	// +optional
	status?: #TokenReviewStatus @go(Status) @protobuf(3,bytes,opt)
}

// TokenReviewSpec is a description of the token authentication request.
#TokenReviewSpec: {
	// Token is the opaque bearer token.
	// +optional
	token?: string @go(Token) @protobuf(1,bytes,opt)

	// Audiences is a list of the identifiers that the resource server presented
	// with the token identifies as. Audience-aware token authenticators will
	// verify that the token was intended for at least one of the audiences in
	// this list. If no audiences are provided, the audience will default to the
	// audience of the Kubernetes apiserver.
	// +optional
	audiences?: [...string] @go(Audiences,[]string) @protobuf(2,bytes,rep)
}

// TokenReviewStatus is the result of the token authentication request.
#TokenReviewStatus: {
	// Authenticated indicates that the token was associated with a known user.
	// +optional
	authenticated?: bool @go(Authenticated) @protobuf(1,varint,opt)

	// User is the UserInfo associated with the provided token.
	// +optional
	user?: #UserInfo @go(User) @protobuf(2,bytes,opt)

	// Audiences are audience identifiers chosen by the authenticator that are
	// compatible with both the TokenReview and token. An identifier is any
	// identifier in the intersection of the TokenReviewSpec audiences and the
	// token's audiences. A client of the TokenReview API that sets the
	// spec.audiences field should validate that a compatible audience identifier
	// is returned in the status.audiences field to ensure that the TokenReview
	// server is audience aware. If a TokenReview returns an empty
	// status.audience field where status.authenticated is "true", the token is
	// valid against the audience of the Kubernetes API server.
	// +optional
	audiences?: [...string] @go(Audiences,[]string) @protobuf(4,bytes,rep)

	// Error indicates that the token couldn't be checked
	// +optional
	error?: string @go(Error) @protobuf(3,bytes,opt)
}

// UserInfo holds the information about the user needed to implement the
// user.Info interface.
#UserInfo: {
	// The name that uniquely identifies this user among all active users.
	// +optional
	username?: string @go(Username) @protobuf(1,bytes,opt)

	// A unique value that identifies this user across time. If this user is
	// deleted and another user by the same name is added, they will have
	// different UIDs.
	// +optional
	uid?: string @go(UID) @protobuf(2,bytes,opt)

	// The names of groups this user is a part of.
	// +optional
	groups?: [...string] @go(Groups,[]string) @protobuf(3,bytes,rep)

	// Any additional information provided by the authenticator.
	// +optional
	extra?: {[string]: #ExtraValue} @go(Extra,map[string]ExtraValue) @protobuf(4,bytes,rep)
}

// ExtraValue masks the value so protobuf can generate
// +protobuf.nullable=true
// +protobuf.options.(gogoproto.goproto_stringer)=false
#ExtraValue: [...string]

// TokenRequest requests a token for a given service account.
#TokenRequest: {
	// Spec holds information about the request being evaluated
	spec: #TokenRequestSpec @go(Spec) @protobuf(2,bytes,opt)

	// Status is filled in by the server and indicates whether the token can be authenticated.
	// +optional
	status?: #TokenRequestStatus @go(Status) @protobuf(3,bytes,opt)
}

// TokenRequestSpec contains client provided parameters of a token request.
#TokenRequestSpec: {
	// Audiences are the intendend audiences of the token. A recipient of a
	// token must identitfy themself with an identifier in the list of
	// audiences of the token, and otherwise should reject the token. A
	// token issued for multiple audiences may be used to authenticate
	// against any of the audiences listed but implies a high degree of
	// trust between the target audiences.
	audiences: [...string] @go(Audiences,[]string) @protobuf(1,bytes,rep)

	// ExpirationSeconds is the requested duration of validity of the request. The
	// token issuer may return a token with a different validity duration so a
	// client needs to check the 'expiration' field in a response.
	// +optional
	expirationSeconds?: null | int64 @go(ExpirationSeconds,*int64) @protobuf(4,varint,opt)

	// BoundObjectRef is a reference to an object that the token will be bound to.
	// The token will only be valid for as long as the bound object exists.
	// NOTE: The API server's TokenReview endpoint will validate the
	// BoundObjectRef, but other audiences may not. Keep ExpirationSeconds
	// small if you want prompt revocation.
	// +optional
	boundObjectRef?: null | #BoundObjectReference @go(BoundObjectRef,*BoundObjectReference) @protobuf(3,bytes,opt)
}

// TokenRequestStatus is the result of a token request.
#TokenRequestStatus: {
	// Token is the opaque bearer token.
	token: string @go(Token) @protobuf(1,bytes,opt)
}

// BoundObjectReference is a reference to an object that a token is bound to.
#BoundObjectReference: {
	// Kind of the referent. Valid kinds are 'Pod' and 'Secret'.
	// +optional
	kind?: string @go(Kind) @protobuf(1,bytes,opt)

	// API version of the referent.
	// +optional
	apiVersion?: string @go(APIVersion) @protobuf(2,bytes,opt)

	// Name of the referent.
	// +optional
	name?: string @go(Name) @protobuf(3,bytes,opt)
}
