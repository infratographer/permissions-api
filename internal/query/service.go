package query

import "github.com/authzed/authzed-go/v1"

// Engine represents a client for making permissions queries.
type Engine struct {
	namespace string
	client    *authzed.Client
}

// NewEngine returns a new client for making permissions queries.
func NewEngine(namespace string, client *authzed.Client) *Engine {
	return &Engine{
		namespace: namespace,
		client:    client,
	}
}
