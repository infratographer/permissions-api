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

var errorInvalidNamespace = errors.New("invalid namespace")

var _ query.Engine = &Engine{}

// Engine represents an engine that does nothing and accepts all resource types.
type Engine struct {
	mock.Mock
	Namespace string
	schema    []types.ResourceType
}

// AssignSubjectRole does nothing but satisfies the Engine interface.
func (e *Engine) AssignSubjectRole(context.Context, types.Resource, types.Role) error {
	args := e.Called()

	return args.Error(0)
}

// UnassignSubjectRole does nothing but satisfies the Engine interface.
func (e *Engine) UnassignSubjectRole(context.Context, types.Resource, types.Role) error {
	args := e.Called()

	return args.Error(0)
}

// CreateRelationships does nothing but satisfies the Engine interface.
func (e *Engine) CreateRelationships(context.Context, []types.Relationship) error {
	args := e.Called()

	return args.Error(0)
}

// CreateRole creates a Role object and does not persist it anywhere.
func (e *Engine) CreateRole(context.Context, types.Resource, types.Resource, string, []string) (types.Role, error) {
	args := e.Called()

	retRole := args.Get(0).(types.Role)

	return retRole, args.Error(1)
}

// CreateRoleV2 creates a v2 role object
// TODO: Implement this
func (e *Engine) CreateRoleV2(context.Context, types.Resource, types.Resource, string, []string) (types.Role, error) {
	return types.Role{}, nil
}

// ListRolesV2 list roles
func (e *Engine) ListRolesV2(context.Context, types.Resource) ([]types.Role, error) {
	return nil, nil
}

// UpdateRole returns the provided mock results.
func (e *Engine) UpdateRole(context.Context, types.Resource, types.Resource, string, []string) (types.Role, error) {
	args := e.Called()

	retRole := args.Get(0).(types.Role)

	return retRole, args.Error(1)
}

// UpdateRoleV2 returns nothing but satisfies the Engine interface.
func (e *Engine) UpdateRoleV2(context.Context, types.Resource, types.Resource, string, []string) (types.Role, error) {
	return types.Role{}, nil
}

// GetRole returns nothing but satisfies the Engine interface.
func (e *Engine) GetRole(context.Context, types.Resource) (types.Role, error) {
	args := e.Called()

	retRole := args.Get(0).(types.Role)

	return retRole, args.Error(1)
}

// GetRoleV2 returns nothing but satisfies the Engine interface.
func (e *Engine) GetRoleV2(context.Context, types.Resource) (types.Role, error) {
	return types.Role{}, nil
}

// GetRoleResource returns nothing but satisfies the Engine interface.
func (e *Engine) GetRoleResource(context.Context, types.Resource) (types.Resource, error) {
	args := e.Called()

	retResc := args.Get(0).(types.Resource)

	return retResc, args.Error(1)
}

// ListAssignments returns nothing but satisfies the Engine interface.
func (e *Engine) ListAssignments(context.Context, types.Role) ([]types.Resource, error) {
	args := e.Called()

	ret := args.Get(0).([]types.Resource)

	return ret, args.Error(1)
}

// ListRelationshipsFrom returns nothing but satisfies the Engine interface.
func (e *Engine) ListRelationshipsFrom(context.Context, types.Resource) ([]types.Relationship, error) {
	return nil, nil
}

// ListRelationshipsTo returns nothing but satisfies the Engine interface.
func (e *Engine) ListRelationshipsTo(context.Context, types.Resource) ([]types.Relationship, error) {
	return nil, nil
}

// ListRoles returns nothing but satisfies the Engine interface.
func (e *Engine) ListRoles(context.Context, types.Resource) ([]types.Role, error) {
	return nil, nil
}

// DeleteRelationships does nothing but satisfies the Engine interface.
func (e *Engine) DeleteRelationships(context.Context, ...types.Relationship) error {
	args := e.Called()

	return args.Error(0)
}

// DeleteRole does nothing but satisfies the Engine interface.
func (e *Engine) DeleteRole(context.Context, types.Resource) error {
	args := e.Called()

	return args.Error(0)
}

// DeleteRoleV2 does nothing but satisfies the Engine interface.
func (e *Engine) DeleteRoleV2(context.Context, types.Resource) error {
	return nil
}

// DeleteResourceRelationships does nothing but satisfies the Engine interface.
func (e *Engine) DeleteResourceRelationships(context.Context, types.Resource) error {
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
func (e *Engine) SubjectHasPermission(context.Context, types.Resource, string, types.Resource) error {
	e.Called()

	return nil
}

// CreateRoleBinding returns nothing but satisfies the Engine interface.
func (e *Engine) CreateRoleBinding(context.Context, types.Resource, types.Resource, types.Resource, []types.RoleBindingSubject) (types.RoleBinding, error) {
	return types.RoleBinding{}, nil
}

// ListRoleBindings returns nothing but satisfies the Engine interface.
func (e *Engine) ListRoleBindings(context.Context, types.Resource, *types.Resource) ([]types.RoleBinding, error) {
	return nil, nil
}

// GetRoleBinding returns nothing but satisfies the Engine interface.
func (e *Engine) GetRoleBinding(context.Context, types.Resource) (types.RoleBinding, error) {
	return types.RoleBinding{}, nil
}

// DeleteRoleBinding returns nothing but satisfies the Engine interface.
func (e *Engine) DeleteRoleBinding(context.Context, types.Resource) error {
	return nil
}

// UpdateRoleBinding returns nothing but satisfies the Engine interface.
func (e *Engine) UpdateRoleBinding(context.Context, types.Resource, types.Resource, []types.RoleBindingSubject) (types.RoleBinding, error) {
	return types.RoleBinding{}, nil
}

// GetRoleBindingResource returns nothing but satisfies the Engine interface.
func (e *Engine) GetRoleBindingResource(context.Context, types.Resource) (types.Resource, error) {
	return types.Resource{}, nil
}

// AllActions returns nothing but satisfies the Engine interface.
func (e *Engine) AllActions() []string {
	return nil
}
