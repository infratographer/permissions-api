package mock

import (
	"context"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/types"

	"github.com/stretchr/testify/mock"
	"go.infratographer.com/x/gidx"
)

var (
	_ query.Engine = &Engine{}
)

// Engine represents an engine that does nothing and accepts all resource types.
type Engine struct {
	mock.Mock
	Namespace string
}

// AssignSubjectRole does nothing but satisfies the Engine interface.
func (e *Engine) AssignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) (string, error) {
	return "", nil
}

// CreateRelationships does nothing but satisfies the Engine interface.
func (e *Engine) CreateRelationships(ctx context.Context, rels []types.Relationship) (string, error) {
	args := e.Called()

	return args.String(0), args.Error(1)
}

// CreateRole creates a Role object and does not persist it anywhere.
func (e *Engine) CreateRole(ctx context.Context, res types.Resource, actions []string) (types.Role, string, error) {
	// Copy actions instead of using the given slice
	outActions := make([]string, len(actions))

	copy(outActions, actions)

	role := types.Role{
		ID:      gidx.MustNewID(query.ApplicationPrefix),
		Actions: outActions,
	}

	return role, "", nil
}

// ListAssignments returns nothing but satisfies the Engine interface.
func (e *Engine) ListAssignments(ctx context.Context, role types.Role, queryToken string) ([]types.Resource, error) {
	return nil, nil
}

// ListRelationships returns nothing but satisfies the Engine interface.
func (e *Engine) ListRelationships(ctx context.Context, resource types.Resource, queryToken string) ([]types.Relationship, error) {
	return nil, nil
}

// ListRoles returns nothing but satisfies the Engine interface.
func (e *Engine) ListRoles(ctx context.Context, resource types.Resource, queryToken string) ([]types.Role, error) {
	return nil, nil
}

// DeleteRelationships does nothing but satisfies the Engine interface.
func (e *Engine) DeleteRelationships(ctx context.Context, resource types.Resource) (string, error) {
	args := e.Called()

	return args.String(0), args.Error(1)
}

// NewResourceFromID creates a new resource object based on the given ID.
func (e *Engine) NewResourceFromID(id gidx.PrefixedID) (types.Resource, error) {
	out := types.Resource{
		Type: id.Prefix(),
		ID:   id,
	}

	return out, nil
}

// SubjectHasPermission returns nil to satisfy the Engine interface.
func (e *Engine) SubjectHasPermission(ctx context.Context, subject types.Resource, action string, resource types.Resource, queryToken string) error {
	e.Called()

	return nil
}
