package cmd

import (
	"errors"
)

// ErrUnsupportedDBEngine is returned when an unsupported database engine is provided.
var ErrUnsupportedDBEngine = errors.New("unsupported database engine")
