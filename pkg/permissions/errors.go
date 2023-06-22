package permissions

import "errors"

var (
	// ErrNoAuthToken is the error returned when there is no auth token provided for the API request
	ErrNoAuthToken = errors.New("no auth token provided for client")

	// ErrInvalidAuthToken is the error returned when the auth token is not the expected value
	ErrInvalidAuthToken = errors.New("invalid auth token")

	// ErrPermissionDenied is the error returned when permission is denied to a call
	ErrPermissionDenied = errors.New("subject doesn't have access")

	// ErrBadResponse is the error returned when we receive a bad response from the server
	ErrBadResponse = errors.New("bad response from server")

	// ErrCheckerNotFound is the error returned when CheckAccess does not find the appropriate checker context
	ErrCheckerNotFound = errors.New("no checker found in context")
)
