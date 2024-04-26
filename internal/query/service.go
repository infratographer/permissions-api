package query

import (
	"context"

	"github.com/authzed/authzed-go/v1"
	"github.com/nats-io/nats.go"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/types"
)

const (
	outcomeAllowed = "allowed"
	outcomeDenied  = "denied"

	// DefaultRoleResourceName is the default name for a role resource
	DefaultRoleResourceName = "role"
	// DefaultRoleBindingResourceName is the default name for a role binding resource
	DefaultRoleBindingResourceName = "role_binding"
)

// Engine represents a client for making permissions queries.
type Engine interface {
	AssignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) error
	UnassignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) error
	CreateRelationships(ctx context.Context, rels []types.Relationship) error
	CreateRole(ctx context.Context, actor, res types.Resource, roleName string, actions []string) (types.Role, error)
	UpdateRole(ctx context.Context, actor, roleResource types.Resource, newName string, newActions []string) (types.Role, error)
	GetRole(ctx context.Context, roleResource types.Resource) (types.Role, error)
	GetRoleResource(ctx context.Context, roleResource types.Resource) (types.Resource, error)
	ListAssignments(ctx context.Context, role types.Role) ([]types.Resource, error)
	ListRelationshipsFrom(ctx context.Context, resource types.Resource) ([]types.Relationship, error)
	ListRelationshipsTo(ctx context.Context, resource types.Resource) ([]types.Relationship, error)
	ListRoles(ctx context.Context, resource types.Resource) ([]types.Role, error)
	DeleteRelationships(ctx context.Context, relationships ...types.Relationship) error
	DeleteRole(ctx context.Context, roleResource types.Resource) error
	DeleteResourceRelationships(ctx context.Context, resource types.Resource) error
	NewResourceFromID(id gidx.PrefixedID) (types.Resource, error)
	GetResourceType(name string) *types.ResourceType
	SubjectHasPermission(ctx context.Context, subject types.Resource, action string, resource types.Resource) error

	// v2 functions, add role bindings support

	// CreateRoleV2 creates a v2 role scoped to the given owner resource with the given actions.
	CreateRoleV2(ctx context.Context, actor, owner types.Resource, roleName string, actions []string) (types.Role, error)
	// ListRolesV2 returns all V2 roles owned by the given resource.
	ListRolesV2(ctx context.Context, owner types.Resource) ([]types.Role, error)
	// GetRoleV2 returns a V2 role
	GetRoleV2(ctx context.Context, role types.Resource) (types.Role, error)
	// UpdateRoleV2 updates a V2 role with the given name and actions.
	UpdateRoleV2(ctx context.Context, actor, roleResource types.Resource, newName string, newActions []string) (types.Role, error)
	// DeleteRoleV2 deletes a V2 role.
	DeleteRoleV2(ctx context.Context, roleResource types.Resource) error

	// CreateRoleBinding creates all the necessary relationships for a role binding.
	// role binding here establishes a three-way relationship between a role,
	// a resource, and the subjects.
	CreateRoleBinding(ctx context.Context, actor, resource, role types.Resource, subjects []types.RoleBindingSubject) (types.RoleBinding, error)
	// ListRoleBindings lists all role-bindings for a resource, an optional Role
	// can be provided to filter the role-bindings.
	ListRoleBindings(ctx context.Context, resource types.Resource, optionalRole *types.Resource) ([]types.RoleBinding, error)
	// GetRoleBinding fetches a role-binding by its ID.
	GetRoleBinding(ctx context.Context, rolebinding types.Resource) (types.RoleBinding, error)
	// UpdateRoleBinding updates the subjects of a role-binding.
	UpdateRoleBinding(ctx context.Context, actor, rolebinding types.Resource, subjects []types.RoleBindingSubject) (types.RoleBinding, error)
	// DeleteRoleBinding removes subjects from a role-binding.
	DeleteRoleBinding(ctx context.Context, rolebinding types.Resource) error
	// GetRoleBindingResource fetches the resource to which a role-binding
	// belongs
	GetRoleBindingResource(ctx context.Context, rb types.Resource) (types.Resource, error)

	AllActions() []string
}

type engine struct {
	tracer                   trace.Tracer
	logger                   *zap.SugaredLogger
	namespace                string
	client                   *authzed.Client
	kv                       nats.KeyValue
	store                    storage.Storage
	schema                   []types.ResourceType
	schemaPrefixMap          map[string]types.ResourceType
	schemaTypeMap            map[string]types.ResourceType
	schemaSubjectRelationMap map[string]map[string][]string
	schemaRoleables          []types.ResourceType

	rbac iapl.RBAC
	// rolebindingSubjectsMap maps the name of the role-binding subject to the target type
	// and provide quick lookups for the role-binding subjects.
	rolebindingSubjectsMap map[string]types.TargetType
	// rbacV2ResourceTypes is a list of resource types that had rbac V2 enabled,
	// role-binding only works with resource types that are in this list
	rbacV2ResourceTypes []types.ResourceType
}

func (e *engine) cacheSchemaResources() {
	e.schemaPrefixMap = make(map[string]types.ResourceType, len(e.schema))
	e.schemaTypeMap = make(map[string]types.ResourceType, len(e.schema))
	e.schemaSubjectRelationMap = make(map[string]map[string][]string)
	e.schemaRoleables = []types.ResourceType{}
	e.rolebindingSubjectsMap = make(map[string]types.TargetType, len(e.rbac.RoleBindingSubjects))
	e.rbacV2ResourceTypes = []types.ResourceType{}

	for _, res := range e.schema {
		e.schemaPrefixMap[res.IDPrefix] = res
		e.schemaTypeMap[res.Name] = res

		for _, relationship := range res.Relationships {
			for _, t := range relationship.Types {
				if _, ok := e.schemaSubjectRelationMap[t.Name]; !ok {
					e.schemaSubjectRelationMap[t.Name] = make(map[string][]string)
				}

				e.schemaSubjectRelationMap[t.Name][relationship.Relation] = append(e.schemaSubjectRelationMap[t.Name][relationship.Relation], res.Name)
			}
		}

		if resourceHasRoleBindings(res) {
			e.schemaRoleables = append(e.schemaRoleables, res)
		}

		if rb := resourceHasRoleBindingV2(res); rb != nil {
			e.rbacV2ResourceTypes = append(e.rbacV2ResourceTypes, res)
		}
	}

	for _, subj := range e.rbac.RoleBindingSubjects {
		e.rolebindingSubjectsMap[subj.Name] = subj
	}
}

func resourceHasRoleBindings(resType types.ResourceType) bool {
	for _, action := range resType.Actions {
		for _, cond := range action.Conditions {
			if cond.RoleBinding != nil {
				return true
			}
		}
	}

	return false
}

func resourceHasRoleBindingV2(resType types.ResourceType) *types.ConditionRoleBindingV2 {
	for _, action := range resType.Actions {
		for _, cond := range action.Conditions {
			if cond.RoleBindingV2 != nil {
				return cond.RoleBindingV2
			}
		}
	}

	return nil
}

// NewEngine returns a new client for making permissions queries.
func NewEngine(namespace string, client *authzed.Client, kv nats.KeyValue, store storage.Storage, options ...Option) (Engine, error) {
	tracer := otel.GetTracerProvider().Tracer("go.infratographer.com/permissions-api/internal/query")

	e := &engine{
		logger:    zap.NewNop().Sugar(),
		namespace: namespace,
		client:    client,
		kv:        kv,
		store:     store,
		tracer:    tracer,
	}

	for _, fn := range options {
		fn(e)
	}

	if e.schema == nil {
		p := iapl.DefaultPolicy()
		e.schema = p.Schema()
		e.rbac = iapl.RBAC{}

		e.cacheSchemaResources()
	}

	return e, nil
}

// Option is a functional option for the engine
type Option func(*engine)

// WithLogger sets the logger for the engine
func WithLogger(logger *zap.SugaredLogger) Option {
	return func(e *engine) {
		e.logger = logger
	}
}

// WithPolicy sets the policy for the engine
func WithPolicy(policy iapl.Policy) Option {
	return func(e *engine) {
		e.schema = policy.Schema()

		rbac := policy.RBAC()
		if rbac == nil {
			e.rbac = iapl.RBAC{}
		} else {
			e.rbac = *rbac
		}

		e.cacheSchemaResources()
	}
}
