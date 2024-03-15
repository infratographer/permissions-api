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
func (e *Engine) AssignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) error {
	args := e.Called()

	return args.Error(0)
}

// UnassignSubjectRole does nothing but satisfies the Engine interface.
func (e *Engine) UnassignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) error {
	args := e.Called()

	return args.Error(0)
}

// CreateRelationships does nothing but satisfies the Engine interface.
func (e *Engine) CreateRelationships(ctx context.Context, rels []types.Relationship) error {
	args := e.Called()

	return args.Error(0)
}

// CreateRole creates a Role object and does not persist it anywhere.
func (e *Engine) CreateRole(ctx context.Context, actor, res types.Resource, name string, actions []string) (types.Role, error) {
	args := e.Called()

	retRole := args.Get(0).(types.Role)

	return retRole, args.Error(1)
}

// UpdateRole returns the provided mock results.
func (e *Engine) UpdateRole(ctx context.Context, actor, roleResource types.Resource, newName string, newActions []string) (types.Role, error) {
	args := e.Called()

	retRole := args.Get(0).(types.Role)

	return retRole, args.Error(1)
}

// GetRole returns nothing but satisfies the Engine interface.
func (e *Engine) GetRole(ctx context.Context, roleResource types.Resource) (types.Role, error) {
	args := e.Called()

	retRole := args.Get(0).(types.Role)

	return retRole, args.Error(1)
}

// GetRoleResource returns nothing but satisfies the Engine interface.
func (e *Engine) GetRoleResource(ctx context.Context, roleResource types.Resource) (types.Resource, error) {
	args := e.Called()

	retResc := args.Get(0).(types.Resource)

	return retResc, args.Error(1)
}

// ListAssignments returns nothing but satisfies the Engine interface.
func (e *Engine) ListAssignments(ctx context.Context, role types.Role) ([]types.Resource, error) {
	args := e.Called()

	ret := args.Get(0).([]types.Resource)

	return ret, args.Error(1)
}

// ListRelationshipsFrom returns nothing but satisfies the Engine interface.
func (e *Engine) ListRelationshipsFrom(ctx context.Context, resource types.Resource) ([]types.Relationship, error) {
	return nil, nil
}

// ListRelationshipsTo returns nothing but satisfies the Engine interface.
func (e *Engine) ListRelationshipsTo(ctx context.Context, resource types.Resource) ([]types.Relationship, error) {
	return nil, nil
}

// ListRoles returns nothing but satisfies the Engine interface.
func (e *Engine) ListRoles(ctx context.Context, resource types.Resource) ([]types.Role, error) {
	return nil, nil
}

// DeleteRelationships does nothing but satisfies the Engine interface.
func (e *Engine) DeleteRelationships(ctx context.Context, relationships ...types.Relationship) error {
	args := e.Called()

	return args.Error(0)
}

// DeleteRole does nothing but satisfies the Engine interface.
func (e *Engine) DeleteRole(ctx context.Context, roleResource types.Resource) error {
	args := e.Called()

	return args.Error(0)
}

// DeleteResourceRelationships does nothing but satisfies the Engine interface.
func (e *Engine) DeleteResourceRelationships(ctx context.Context, resource types.Resource) error {
	args := e.Called()

	return args.Error(0)
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
func (e *Engine) SubjectHasPermission(ctx context.Context, subject types.Resource, action string, resource types.Resource) error {
	e.Called()

	return nil
}
