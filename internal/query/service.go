package query

import "github.com/authzed/authzed-go/v1"

// Engine represents a client for making permissions queries.
type Engine struct {
	namespace string
	client    *authzed.Client
}

func NewEngine(namespace string, client *authzed.Client) *Engine {
	return &Engine{
		namespace: namespace,
		client:    client,
	}
}
