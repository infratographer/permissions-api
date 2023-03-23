package query

import "errors"

var (
	// ErrScopeNotAssigned represents an error condition where the actor is not able to complete
	// the given request.
	ErrScopeNotAssigned = errors.New("the actor does not have permissions to complete this request")
)
