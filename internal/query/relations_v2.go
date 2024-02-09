package query

import (
	"context"
	"strings"
	"sync"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/types"
)

// V2 Role and Role Bindings

const roleOwnerRelation = "owner"

func (e *engine) namespaced(name string) string {
	return e.namespace + "/" + name
}

// CreateRoleV2 creates a v2 role scoped to the given resource with the given actions.
func (e *engine) CreateRoleV2(ctx context.Context, actor, owner types.Resource, roleName string, actions []string) (types.Role, error) {
	ctx, span := e.tracer.Start(ctx, "engine.CreateRoleV2")

	defer span.End()

	roleName = strings.TrimSpace(roleName)

	role := newRoleWithPrefix(e.schemaTypeMap[e.rbac.RoleResource].IDPrefix, roleName, actions)
	roleRels := e.roleV2Relationships(role)
	roleRels = append(roleRels, e.roleV2OwnerRelationship(role, owner))

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
		return types.Role{}, nil
	}

	dbRole, err := e.store.CreateRole(dbCtx, actor.ID, role.ID, roleName, owner.ID)
	if err != nil {
		return types.Role{}, err
	}

	request := &pb.WriteRelationshipsRequest{Updates: roleRels}

	if _, err := e.client.WriteRelationships(ctx, request); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.Role{}, err
	}

	if err = e.store.CommitContext(dbCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		// No rollback of spicedb relations are done here.
		// This does result in dangling unused entries in spicedb,
		// however there are no assignments to these newly created
		// and now discarded roles and so they won't be used.

		return types.Role{}, err
	}

	role.CreatedBy = dbRole.CreatedBy
	role.UpdatedBy = dbRole.UpdatedBy
	role.ResourceID = dbRole.ResourceID
	role.CreatedAt = dbRole.CreatedAt
	role.UpdatedAt = dbRole.UpdatedAt

	return role, nil
}

// ListRolesV2 returns all V2 roles owned by the given resource.
func (e *engine) ListRolesV2(ctx context.Context, owner types.Resource) ([]types.Role, error) {
	const ListRolesErrBufLen = 2

	var (
		spicedbRoles []types.Role
		rolesByID    map[gidx.PrefixedID]storage.Role
		wg           = &sync.WaitGroup{}
		errs         = make(chan error, ListRolesErrBufLen)
	)

	ctx, span := e.tracer.Start(
		ctx,
		"ListRolesV2",
		trace.WithAttributes(
			attribute.Stringer(
				"owner",
				owner.ID,
			),
		),
	)
	defer span.End()

	// 1. list roles from spice DB
	wg.Add(1)

	go func() {
		defer wg.Done()

		roles, err := e.listSpicedbRolesV2(ctx, owner)
		if err != nil {
			errs <- err
			return
		}

		spicedbRoles = roles
	}()

	// 2. build roles map from permission-api DB
	wg.Add(1)

	go func() {
		defer wg.Done()

		apidbctx, span := e.tracer.Start(ctx, "listRolesFromPermissionAPI")
		defer span.End()

		roles, err := e.store.ListResourceRoles(apidbctx, owner.ID)
		if err != nil {
			errs <- err
			return
		}

		dbRoles := roles
		rolesByID = make(map[gidx.PrefixedID]storage.Role, len(dbRoles))

		for _, role := range dbRoles {
			rolesByID[role.ID] = role
		}
	}()

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
	}

	// 3. build a list of roles with data from both DBs
	for i, spicedbRole := range spicedbRoles {
		dbRole := rolesByID[spicedbRole.ID]

		spicedbRoles[i] = types.Role{
			ID:         dbRole.ID,
			Name:       dbRole.Name,
			Actions:    spicedbRole.Actions,
			ResourceID: dbRole.ResourceID,
			CreatedBy:  dbRole.CreatedBy,
			UpdatedBy:  dbRole.UpdatedBy,
			CreatedAt:  dbRole.CreatedAt,
			UpdatedAt:  dbRole.UpdatedAt,
		}
	}

	return spicedbRoles, nil
}

// roleV2OwnerRelationship creates a relationship between a V2 role and its owner.
func (e *engine) roleV2OwnerRelationship(role types.Role, owner types.Resource) *pb.RelationshipUpdate {
	roleResource, err := e.NewResourceFromID(role.ID)
	if err != nil {
		panic(err)
	}

	roleResourceType := e.GetResourceType(e.rbac.RoleResource)
	if roleResourceType == nil {
		return nil
	}

	roleRef := resourceToSpiceDBRef(e.namespace, roleResource)
	ownerRef := resourceToSpiceDBRef(e.namespace, owner)

	return &pb.RelationshipUpdate{
		Operation: pb.RelationshipUpdate_OPERATION_TOUCH,
		Relationship: &pb.Relationship{
			Resource: roleRef,
			Relation: roleOwnerRelation,
			Subject: &pb.SubjectReference{
				Object: ownerRef,
			},
		},
	}
}

// roleV2Relationships creates relationships between a V2 role and its permissions.
func (e *engine) roleV2Relationships(role types.Role) []*pb.RelationshipUpdate {
	var rels []*pb.RelationshipUpdate

	roleResource, err := e.NewResourceFromID(role.ID)
	if err != nil {
		panic(err)
	}

	roleResourceType := e.GetResourceType(e.rbac.RoleResource)
	if roleResourceType == nil {
		return rels
	}

	roleRef := resourceToSpiceDBRef(e.namespace, roleResource)

	// creates permission relationship line in role
	// e.g., role:<role_name>#<action>_rel@<namespace>/<subjType>:*
	createRelationshipsForAction := func(action string) {
		for _, subjType := range e.rbac.RoleRelationshipSubjects {
			e.logger.Debugf("creating permission rel for action: %s, subjType: %s\n", action, subjType)
			rels = append(rels, &pb.RelationshipUpdate{
				Operation: pb.RelationshipUpdate_OPERATION_TOUCH,
				Relationship: &pb.Relationship{
					Resource: roleRef,
					Relation: actionToRelation(action),
					Subject: &pb.SubjectReference{
						Object: &pb.ObjectReference{
							ObjectType: e.namespaced(subjType),
							ObjectId:   "*",
						},
					},
				},
			})
		}
	}

	for _, action := range role.Actions {
		createRelationshipsForAction(action)
	}

	return rels
}

func (e *engine) listSpicedbRolesV2(ctx context.Context, owner types.Resource) ([]types.Role, error) {
	ctx, span := e.tracer.Start(ctx, "listSpicedbRolesV2")
	defer span.End()

	ownerType := e.namespaced(owner.Type)
	roleType := e.namespaced(e.rbac.RoleResource)

	filter := &pb.RelationshipFilter{
		ResourceType:     roleType,
		OptionalRelation: roleOwnerRelation,
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType:       ownerType,
			OptionalSubjectId: owner.ID.String(),
		},
	}

	relationships, err := e.readRelationships(ctx, filter)
	if err != nil {
		return nil, err
	}

	spicedbRoles := make([]types.Role, len(relationships))
	errs := make(chan error, len(relationships))
	wg := &sync.WaitGroup{}

	for i, rel := range relationships {
		wg.Add(1)

		go func(index int, role *pb.ObjectReference) {
			defer wg.Done()

			actions, err := e.listRoleV2Actions(ctx, role)
			if err != nil {
				errs <- err
				return
			}

			roleID, err := gidx.Parse(role.ObjectId)
			if err != nil {
				errs <- err
				return
			}

			spicedbRoles[index] = types.Role{
				ID:      roleID,
				Actions: actions,
			}
		}(i, rel.Resource)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
	}

	return spicedbRoles, nil
}

func (e *engine) listRoleV2Actions(ctx context.Context, role *pb.ObjectReference) ([]string, error) {
	if len(role.ObjectType) == 0 {
		return nil, nil
	}

	// there could be multiple subjects for a permission,
	// e.g.
	//   infratographer/rolev2:lb_viewer#loadbalancer_get_rel@infratographer/user:*
	//   infratographer/rolev2:lb_viewer#loadbalancer_get_rel@infratographer/client:*
	// here we only need one of them
	permRelationshipSubjType := e.namespaced(e.rbac.RoleRelationshipSubjects[0])

	filter := &pb.RelationshipFilter{
		ResourceType:       e.namespaced(e.rbac.RoleResource),
		OptionalResourceId: role.ObjectId,
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType:       permRelationshipSubjType,
			OptionalSubjectId: "*",
		},
	}

	relationships, err := e.readRelationships(ctx, filter)
	if err != nil {
		return nil, err
	}

	e.logger.Debugf("listing %d actions for %s: %s", len(relationships), e.namespaced(e.rbac.RoleResource), role.ObjectId)

	actions := make([]string, len(relationships))

	for i, rel := range relationships {
		actions[i] = relationToAction(rel.Relation)
	}

	return actions, nil
}
