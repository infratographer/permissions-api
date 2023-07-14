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
	UnassignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) (string, error)
	CreateRelationships(ctx context.Context, rels []types.Relationship) (string, error)
	CreateRole(ctx context.Context, res types.Resource, actions []string) (types.Role, string, error)
	ListAssignments(ctx context.Context, role types.Role, queryToken string) ([]types.Resource, error)
	ListRelationshipsFrom(ctx context.Context, resource types.Resource, queryToken string) ([]types.Relationship, error)
	ListRelationshipsTo(ctx context.Context, resource types.Resource, queryToken string) ([]types.Relationship, error)
	ListRoles(ctx context.Context, resource types.Resource, queryToken string) ([]types.Role, error)
	DeleteRelationship(ctx context.Context, rel types.Relationship) (string, error)
	DeleteRole(ctx context.Context, roleResource types.Resource, queryToken string) (string, error)
	DeleteRelationships(ctx context.Context, resource types.Resource) (string, error)
	NewResourceFromID(id gidx.PrefixedID) (types.Resource, error)
	GetResourceType(name string) *types.ResourceType
	SubjectHasPermission(ctx context.Context, subject types.Resource, action string, resource types.Resource) error
}

type engine struct {
	logger                   *zap.SugaredLogger
	namespace                string
	client                   *authzed.Client
	schema                   []types.ResourceType
	schemaPrefixMap          map[string]types.ResourceType
	schemaTypeMap            map[string]types.ResourceType
	schemaSubjectRelationMap map[string]map[string][]string
	schemaRoleables          []types.ResourceType
}

func (e *engine) cacheSchemaResources() {
	e.schemaPrefixMap = make(map[string]types.ResourceType, len(e.schema))
	e.schemaTypeMap = make(map[string]types.ResourceType, len(e.schema))
	e.schemaSubjectRelationMap = make(map[string]map[string][]string)
	e.schemaRoleables = []types.ResourceType{}

	for _, res := range e.schema {
		e.schemaPrefixMap[res.IDPrefix] = res
		e.schemaTypeMap[res.Name] = res

		for _, relationship := range res.Relationships {
			for _, t := range relationship.Types {
				if _, ok := e.schemaSubjectRelationMap[t]; !ok {
					e.schemaSubjectRelationMap[t] = make(map[string][]string)
				}

				e.schemaSubjectRelationMap[t][relationship.Relation] = append(e.schemaSubjectRelationMap[t][relationship.Relation], res.Name)
			}
		}

		if resourceHasRoleBindings(res) {
			e.schemaRoleables = append(e.schemaRoleables, res)
		}
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

// NewEngine returns a new client for making permissions queries.
func NewEngine(namespace string, client *authzed.Client, options ...Option) Engine {
	e := &engine{
		logger:    zap.NewNop().Sugar(),
		namespace: namespace,
		client:    client,
	}

	for _, fn := range options {
		fn(e)
	}

	if e.schema == nil {
		e.schema = iapl.DefaultPolicy().Schema()

		e.cacheSchemaResources()
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

// WithPolicy sets the policy for the engine
func WithPolicy(policy iapl.Policy) Option {
	return func(e *engine) {
		e.schema = policy.Schema()

		e.cacheSchemaResources()
	}
}
