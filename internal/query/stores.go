package query

import "github.com/authzed/authzed-go/v1"

// Stores represents a SpiceDB store.
type Stores struct {
	SpiceDB       *authzed.Client
	SpiceDBPrefix string
}
