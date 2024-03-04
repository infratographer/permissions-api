package iapl

import "errors"

var (
	// ErrorTypeExists represents an error where a duplicate type or union was declared.
	ErrorTypeExists = errors.New("type already exists")
	// ErrorActionBindingExists represents an error where a duplicate binding between a type and action was declared.
	ErrorActionBindingExists = errors.New("action binding already exists")
	// ErrorUnknownType represents an error where a resource type is unknown in the authorization policy.
	ErrorUnknownType = errors.New("unknown resource type")
	// ErrorInvalidCondition represents an error where an action binding condition is invalid.
	ErrorInvalidCondition = errors.New("invalid condition")
	// ErrorUnknownRelation represents an error where a relation is not defined for a resource type.
	ErrorUnknownRelation = errors.New("unknown relation")
	// ErrorUnknownAction represents an error where an action is not defined.
	ErrorUnknownAction = errors.New("unknown action")
	// ErrorMissingRelationship represents an error where a mandatory relationship is missing.
	ErrorMissingRelationship = errors.New("missing relationship")
	// ErrorDuplicateRBACDefinition represents an error where a duplicate RBAC definition was declared.
	ErrorDuplicateRBACDefinition = errors.New("duplicated RBAC definition")
)
