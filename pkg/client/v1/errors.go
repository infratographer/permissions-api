package permissions

import "errors"

var (
	// ErrNoNextPage is the error returned when there is not an additional page of resources
	ErrMissingURI = errors.New("no uri provided for client")

	// ErrNoAuthToken is the error returned when there is no auth token provided for the API request
	ErrNoAuthToken = errors.New("no auth token provided for client")

	// ErrPermissionDenied is the error returned when permission is denied to a call
	ErrPermissionDenied = errors.New("subject doesn't have access")
)
