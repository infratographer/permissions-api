package query

import "errors"

var (
	// ErrActionNotAssigned represents an error condition where the subject is not able to complete
	// the given request.
	ErrActionNotAssigned = errors.New("the subject does not have permissions to complete this request")

	// ErrInvalidReference represents an error condition where a given SpiceDB object reference is for some reason invalid.
	ErrInvalidReference = errors.New("invalid reference")
)
