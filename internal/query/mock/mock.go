package mock

import (
	"context"
	"errors"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/types"

	"github.com/stretchr/testify/mock"
	"go.infratographer.com/x/gidx"
)

var (
	errorInvalidNamespace = errors.New("invalid namespace")
)

var _ query.Engine = &Engine{}

// Engine represents an engine that does nothing and accepts all resource types.
type Engine struct {
	mock.Mock
	Namespace string
	schema    []types.ResourceType
}

// AssignSubjectRole does nothing but satisfies the Engine interface.
func (e *Engine) AssignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) (string, error) {
	return "", nil
}

// UnassignSubjectRole does nothing but satisfies the Engine interface.
func (e *Engine) UnassignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) (string, error) {
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

// DeleteRelationship does nothing but satisfies the Engine interface.
func (e *Engine) DeleteRelationship(ctx context.Context, rel types.Relationship) (string, error) {
	args := e.Called()

	return args.String(0), args.Error(1)
}

// DeleteRole does nothing but satisfies the Engine interface.
func (e *Engine) DeleteRole(ctx context.Context, roleResource types.Resource, queryToken string) (string, error) {
	args := e.Called()

	return args.String(0), args.Error(1)
}

// DeleteRelationships does nothing but satisfies the Engine interface.
func (e *Engine) DeleteRelationships(ctx context.Context, resource types.Resource) (string, error) {
	args := e.Called()

	return args.String(0), args.Error(1)
}

// NewResourceFromID creates a new resource object based on the given ID.
func (e *Engine) NewResourceFromID(id gidx.PrefixedID) (types.Resource, error) {
	prefix := id.Prefix()

	var rType *types.ResourceType

	if e.schema == nil {
		e.schema = iapl.DefaultPolicy().Schema()
	}

	for _, resourceType := range e.schema {
		if resourceType.IDPrefix == prefix {
			rType = &resourceType

			break
		}
	}

	if rType == nil {
		return types.Resource{}, errorInvalidNamespace
	}

	out := types.Resource{
		Type: rType.Name,
		ID:   id,
	}

	return out, nil
}

// GetResourceType returns the resource type by name
func (e *Engine) GetResourceType(name string) *types.ResourceType {
	if e.schema == nil {
		e.schema = iapl.DefaultPolicy().Schema()
	}

	for _, resourceType := range e.schema {
		if resourceType.Name == name {
			return &resourceType
		}
	}

	return nil
}

// SubjectHasPermission returns nil to satisfy the Engine interface.
func (e *Engine) SubjectHasPermission(ctx context.Context, subject types.Resource, action string, resource types.Resource, queryToken string) error {
	e.Called()

	return nil
}
