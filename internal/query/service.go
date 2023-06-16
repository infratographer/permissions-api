package query

import (
	"context"

	"github.com/authzed/authzed-go/v1"
	"go.infratographer.com/x/gidx"

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
	namespace string
	client    *authzed.Client
	schema    []types.ResourceType
}

// NewEngine returns a new client for making permissions queries.
func NewEngine(namespace string, client *authzed.Client) Engine {
	policy := iapl.DefaultPolicy()

	return &engine{
		namespace: namespace,
		client:    client,
		schema:    policy.Schema(),
	}
}
