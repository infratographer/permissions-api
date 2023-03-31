package api

import (
	"errors"
)

// ErrorResponse represents the data that the server will return on any given call
type ErrorResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

var (
	// ErrResourceNotFound is returned when the requested resource isn't found
	ErrResourceNotFound = errors.New("resource not found")

	// ErrSearchNotFound is returned when the requested search term isn't found
	ErrSearchNotFound = errors.New("search term not found")

	// ErrResourceAlreadyExists is returned when the resource already exists
	ErrResourceAlreadyExists = errors.New("resource already exists")
)
