package iapl

import "errors"

var (
	// ErrorUnknownType represents an error where a resource type is unknown in the authorization policy.
	ErrorUnknownType = errors.New("unknown resource type")
	// ErrorInvalidAlias represents an error where a type alias is invalid.
	ErrorInvalidAlias = errors.New("invalid type alias")
	// ErrorInvalidCondition represents an error where an action binding condition is invalid.
	ErrorInvalidCondition = errors.New("invalid condition")
	// ErrorUnknownRelation represents an error where a relation is not defined for a resource type.
	ErrorUnknownRelation = errors.New("unknown relation")
	// ErrorUnknownAction represents an error where an action is not defined.
	ErrorUnknownAction = errors.New("unknown action")
)
