package api

import (
	"errors"
)

// ErrorResponse represents the data that the server will return on any given call.
type ErrorResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

var (
	ErrResourceNotFound      = errors.New("resource not found")
	ErrSearchNotFound        = errors.New("search term not found")
	ErrResourceAlreadyExists = errors.New("resource already exists")
)
