package iapl

import "errors"

var (
	// ErrorTypeExists represents an error where a duplicate type or union was declared.
	ErrorTypeExists = errors.New("type already exists")
	// ErrorUnknownType represents an error where a resource type is unknown in the authorization policy.
	ErrorUnknownType = errors.New("unknown resource type")
	// ErrorInvalidCondition represents an error where an action binding condition is invalid.
	ErrorInvalidCondition = errors.New("invalid condition")
	// ErrorUnknownRelation represents an error where a relation is not defined for a resource type.
	ErrorUnknownRelation = errors.New("unknown relation")
	// ErrorUnknownAction represents an error where an action is not defined.
	ErrorUnknownAction = errors.New("unknown action")
)
