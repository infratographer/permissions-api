package spicedbx

import "errors"

var (
	// ErrorNoNamespace is returned when no namespace is provided with a query
	ErrorNoNamespace = errors.New("no namespace provided")
)
