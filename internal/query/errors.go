package query

import "errors"

var (
	// ErrActionNotAssigned represents an error condition where the subject is not able to complete
	// the given request.
	ErrActionNotAssigned = errors.New("the subject does not have permissions to complete this request")

	// ErrInvalidReference represents an error condition where a given SpiceDB object reference is for some reason invalid.
	ErrInvalidReference = errors.New("invalid reference")

	// ErrInvalidNamespace represents an error when the id prefix is not found in the resource schema
	ErrInvalidNamespace = errors.New("invalid namespace")

	// ErrInvalidType represents an error when a resource type is not found in the resource schema
	ErrInvalidType = errors.New("invalid type")

	// ErrInvalidRelationship represents an error when no matching relationship was found
	ErrInvalidRelationship = errors.New("invalid relationship")

	// ErrRoleNotFound represents an error when no matching role was found on resource
	ErrRoleNotFound = errors.New("role not found")

	// ErrRoleHasTooManyResources represents an error which a role has too many resources
	ErrRoleHasTooManyResources = errors.New("role has too many resources")

	// ErrRoleAlreadyExists represents an error when a role already exists
	ErrRoleAlreadyExists = errors.New("role already exists")
)
