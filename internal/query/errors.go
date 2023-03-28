package query

import "errors"

var (
	// ErrActionNotAssigned represents an error condition where the subject is not able to complete
	// the given request.
	ErrActionNotAssigned = errors.New("the subject does not have permissions to complete this request")
)
