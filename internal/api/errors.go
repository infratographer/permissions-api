package api

import "errors"

var (
	// ErrInvalidID is returned when the ID is invalid
	ErrInvalidID = errors.New("invalid ID")
	// ErrParsingRequestBody is returned when failing to parse the request body
	ErrParsingRequestBody = errors.New("error parsing request body")
)
