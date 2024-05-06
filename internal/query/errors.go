package query

import (
	"errors"
	"fmt"
)

var (
	// ErrActionNotAssigned represents an error condition where the subject is not able to complete
	// the given request.
	ErrActionNotAssigned = errors.New("the subject does not have permissions to complete this request")

	// ErrInvalidAction represents an error condition where the action provided is not valid for the provided resource.
	ErrInvalidAction = errors.New("invalid action for resource")

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

	// ErrResourceNotFound represents an error when no matching resource was found
	ErrResourceNotFound = errors.New("resource not found")

	// ErrRoleBindingNotFound represents an error when no matching role binding was found
	ErrRoleBindingNotFound = errors.New("role binding not found")

	// ErrRoleHasTooManyResources represents an error which a role has too many resources
	ErrRoleHasTooManyResources = errors.New("role has too many resources")

	// ErrInvalidArgument represents an error when there is an invalid argument passed to a function
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrRoleV2ResourceNotDefined is returned when a role v2 resource is not defined
	// in the policy
	ErrRoleV2ResourceNotDefined = errors.New("role v2 resource not defined")

	// ErrDeleteRoleInUse represents an error when a role is in use and cannot be deleted
	ErrDeleteRoleInUse = fmt.Errorf("%w: role is in use", ErrInvalidArgument)

	// ErrRoleAlreadyExists represents an error when a role already exists
	ErrRoleAlreadyExists = fmt.Errorf("%w: role already exists", ErrInvalidArgument)

	// ErrInvalidRoleBindingSubjectType represents an error when a role binding subject type is invalid
	ErrInvalidRoleBindingSubjectType = fmt.Errorf("%w: invalid role binding subject type", ErrInvalidArgument)

	// ErrResourceDoesNotSupportRoleBindingV2 represents an error when a role binding
	// request attempts to use a resource that does not support role binding v2
	ErrResourceDoesNotSupportRoleBindingV2 = fmt.Errorf("%w: resource does not support role binding v2", ErrInvalidArgument)

	// ErrCreateRoleBindingWithNoSubjects represents an error when a role
	// binding is created with no subjects
	ErrCreateRoleBindingWithNoSubjects = fmt.Errorf("%w: role binding must have at least one subject", ErrInvalidArgument)

	// ErrRoleBindingHasNoRelationships represents an internal error when a
	// role binding has no relationships
	ErrRoleBindingHasNoRelationships = errors.New("role binding has no relationships")
)
