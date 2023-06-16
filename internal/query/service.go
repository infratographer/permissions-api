package query

import (
	"context"

	"github.com/authzed/authzed-go/v1"
	"go.infratographer.com/x/gidx"
	"go.uber.org/zap"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/types"
)

// Engine represents a client for making permissions queries.
type Engine interface {
	AssignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) (string, error)
	CreateRelationships(ctx context.Context, rels []types.Relationship) (string, error)
	CreateRole(ctx context.Context, res types.Resource, actions []string) (types.Role, string, error)
	ListAssignments(ctx context.Context, role types.Role, queryToken string) ([]types.Resource, error)
	ListRelationships(ctx context.Context, resource types.Resource, queryToken string) ([]types.Relationship, error)
	ListRoles(ctx context.Context, resource types.Resource, queryToken string) ([]types.Role, error)
	DeleteRelationships(ctx context.Context, resource types.Resource) (string, error)
	NewResourceFromID(id gidx.PrefixedID) (types.Resource, error)
	SubjectHasPermission(ctx context.Context, subject types.Resource, action string, resource types.Resource, queryToken string) error
}

type engine struct {
	logger    *zap.SugaredLogger
	namespace string
	client    *authzed.Client
	schema    []types.ResourceType
}

// NewEngine returns a new client for making permissions queries.
func NewEngine(namespace string, client *authzed.Client, options ...Option) Engine {
	policy := iapl.DefaultPolicy()

	e := &engine{
		logger:    zap.NewNop().Sugar(),
		namespace: namespace,
		client:    client,
		schema:    policy.Schema(),
	}

	for _, fn := range options {
		fn(e)
	}

	return e
}

// Option is a functional option for the engine
type Option func(*engine)

// WithLogger sets the logger for the engine
func WithLogger(logger *zap.SugaredLogger) Option {
	return func(e *engine) {
		e.logger = logger
	}
}
