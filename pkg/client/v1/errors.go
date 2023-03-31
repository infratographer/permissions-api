package permissions

import "errors"

var (
	// ErrMissingURI is the error returned when there uri provided to the client
	ErrMissingURI = errors.New("no uri provided for client")

	// ErrNoAuthToken is the error returned when there is no auth token provided for the API request
	ErrNoAuthToken = errors.New("no auth token provided for client")

	// ErrPermissionDenied is the error returned when permission is denied to a call
	ErrPermissionDenied = errors.New("subject doesn't have access")

	// ErrBadResponse is the error returned when we receive a bad response from the server
	ErrBadResponse = errors.New("bad response from server")
)
