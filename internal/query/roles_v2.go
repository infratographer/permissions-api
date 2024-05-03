package query

import (
	"context"
	"errors"
	"fmt"
	"io"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/types"
)

// V2 Role and Role Bindings

func (e *engine) namespaced(name string) string {
	return e.namespace + "/" + name
}

func (e *engine) CreateRoleV2(ctx context.Context, actor, owner types.Resource, roleName string, actions []string) (types.Role, error) {
	ctx, span := e.tracer.Start(ctx, "engine.CreateRoleV2")

	defer span.End()

	role, err := newRoleWithPrefix(e.schemaTypeMap[e.rbac.RoleResource.Name].IDPrefix, roleName, actions)
	if err != nil {
		return types.Role{}, err
	}

	roleRels, err := e.roleV2Relationships(role)
	if err != nil {
		return types.Role{}, err
	}

	ownerRels, err := e.roleV2OwnerRelationship(role, owner)
	if err != nil {
		return types.Role{}, err
	}

	roleRels = append(roleRels, ownerRels...)

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
		return types.Role{}, nil
	}

	dbRole, err := e.store.CreateRole(dbCtx, actor.ID, role.ID, roleName, owner.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

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

func (e *engine) ListRolesV2(ctx context.Context, owner types.Resource) ([]types.Role, error) {
	ctx, span := e.tracer.Start(
		ctx,
		"engine.ListRolesV2",
		trace.WithAttributes(
			attribute.Stringer(
				"owner",
				owner.ID,
			),
		),
	)
	defer span.End()

	if _, ok := e.rbac.RoleOwnersSet()[owner.Type]; !ok {
		err := fmt.Errorf("%w: %s is not a valid role owner", ErrInvalidType, owner.Type)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return nil, err
	}

	lookupClient, err := e.client.LookupSubjects(ctx, &pb.LookupSubjectsRequest{
		Consistency: &pb.Consistency{
			Requirement: &pb.Consistency_FullyConsistent{
				FullyConsistent: true,
			},
		},
		Resource:          resourceToSpiceDBRef(e.namespace, owner),
		Permission:        iapl.AvailableRolesList,
		SubjectObjectType: e.namespaced(e.rbac.RoleResource.Name),
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return nil, err
	}

	roleIDs := []gidx.PrefixedID{}

	for {
		lookup, err := lookupClient.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}

			break
		}

		id, err := gidx.Parse(lookup.Subject.SubjectObjectId)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			continue
		}

		roleIDs = append(roleIDs, id)
	}

	storageRoles, err := e.store.BatchGetRoleByID(ctx, roleIDs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return nil, err
	}

	roles := make([]types.Role, len(storageRoles))

	for i, r := range storageRoles {
		roles[i] = types.Role{
			Name: r.Name,
			ID:   r.ID,
		}
	}

	return roles, nil
}

func (e *engine) GetRoleV2(ctx context.Context, role types.Resource) (types.Role, error) {
	ctx, span := e.tracer.Start(
		ctx,
		"engine.GetRoleV2",
		trace.WithAttributes(attribute.Stringer("permissions.role_id", role.ID)),
	)
	defer span.End()

	// check if the role is a valid v2 role
	if role.Type != e.rbac.RoleResource.Name {
		err := fmt.Errorf("%w: %s is not a valid v2 Role", ErrInvalidType, role.Type)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.Role{}, err
	}

	// 1. Get role actions from spice DB

	actions, err := e.listRoleV2Actions(ctx, types.Role{ID: role.ID})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.Role{}, err
	}

	// 2. Get role info (name, created_by, etc.) from permissions API DB
	dbrole, err := e.store.GetRoleByID(ctx, role.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.Role{}, err
	}

	resp := types.Role{
		ID:      dbrole.ID,
		Name:    dbrole.Name,
		Actions: actions,

		ResourceID: dbrole.ResourceID,
		CreatedBy:  dbrole.CreatedBy,
		UpdatedBy:  dbrole.UpdatedBy,
		CreatedAt:  dbrole.CreatedAt,
		UpdatedAt:  dbrole.UpdatedAt,
	}

	return resp, nil
}

func (e *engine) UpdateRoleV2(ctx context.Context, actor, roleResource types.Resource, newName string, newActions []string) (types.Role, error) {
	ctx, span := e.tracer.Start(ctx, "engine.UpdateRoleV2")
	defer span.End()

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
		return types.Role{}, err
	}

	err = e.store.LockRoleForUpdate(dbCtx, roleResource.ID)
	if err != nil {
		sErr := fmt.Errorf("failed to lock role: %s: %w", roleResource.ID, err)

		span.RecordError(sErr)
		span.SetStatus(codes.Error, sErr.Error())

		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.Role{}, err
	}

	role, err := e.GetRoleV2(dbCtx, roleResource)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.Role{}, err
	}

	if newName == "" {
		newName = role.Name
	}

	addActions, rmActions := diff(role.Actions, newActions)

	// If no changes, return existing role
	if newName == role.Name && len(addActions) == 0 && len(rmActions) == 0 {
		if err = e.store.CommitContext(dbCtx); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return types.Role{}, err
		}

		return role, nil
	}

	// 1. update role in permissions-api DB
	dbRole, err := e.store.UpdateRole(dbCtx, actor.ID, role.ID, newName)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.Role{}, err
	}

	// 2. update permissions relationships in SpiceDB
	updates := []*pb.RelationshipUpdate{}
	roleRef := resourceToSpiceDBRef(e.namespace, roleResource)

	// 2.a remove old actions
	for _, action := range rmActions {
		updates = append(
			updates,
			e.createRoleV2RelationshipUpdatesForAction(
				action, roleRef,
				pb.RelationshipUpdate_OPERATION_DELETE,
			)...,
		)
	}

	// 2.b add new actions
	for _, action := range addActions {
		updates = append(
			updates,
			e.createRoleV2RelationshipUpdatesForAction(
				action, roleRef,
				pb.RelationshipUpdate_OPERATION_TOUCH,
			)...,
		)
	}

	// 2.c write updates to SpiceDB
	request := &pb.WriteRelationshipsRequest{Updates: updates}

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

		// At this point, SpiceDB changes have already been applied.
		// Attempting to rollback could result in failures that could result in the same situation.
		//
		// TODO: add SpiceDB rollback logic along with rollback failure scenarios.

		return types.Role{}, err
	}

	role.Name = dbRole.Name
	role.CreatedBy = dbRole.CreatedBy
	role.UpdatedBy = dbRole.UpdatedBy
	role.ResourceID = dbRole.ResourceID
	role.CreatedAt = dbRole.CreatedAt
	role.UpdatedAt = dbRole.UpdatedAt
	role.Actions = newActions

	return role, nil
}

func (e *engine) DeleteRoleV2(ctx context.Context, roleResource types.Resource) error {
	ctx, span := e.tracer.Start(ctx, "engine.DeleteRoleV2")
	defer span.End()

	// find all the bindings for the role
	findBindingsFilter := &pb.RelationshipFilter{
		ResourceType:     e.namespaced(e.rbac.RoleBindingResource.Name),
		OptionalRelation: iapl.RolebindingRoleRelation,
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType:       e.namespaced(e.rbac.RoleResource.Name),
			OptionalSubjectId: roleResource.ID.String(),
		},
	}

	bindings, err := e.readRelationships(ctx, findBindingsFilter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	// reject delete if role is in use
	if len(bindings) > 0 {
		err := fmt.Errorf("%w: cannot delete role", ErrDeleteRoleInUse)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	err = e.store.LockRoleForUpdate(dbCtx, roleResource.ID)
	if err != nil {
		sErr := fmt.Errorf("failed to lock role: %s: %w", roleResource.ID, err)

		span.RecordError(sErr)
		span.SetStatus(codes.Error, sErr.Error())

		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	dbRole, err := e.store.GetRoleByID(dbCtx, roleResource.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	roleOwner, err := e.NewResourceFromID(dbRole.ResourceID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	// 1. delete role from permission-api DB
	if _, err = e.store.DeleteRole(dbCtx, roleResource.ID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	// 2. delete role relationships from spice db
	errs := []error{}

	// 2.a remove all relationships from this role

	delRoleRelationshipReq := &pb.DeleteRelationshipsRequest{
		RelationshipFilter: &pb.RelationshipFilter{
			ResourceType:       e.namespaced(e.rbac.RoleResource.Name),
			OptionalResourceId: roleResource.ID.String(),
		},
	}

	if _, err := e.client.DeleteRelationships(ctx, delRoleRelationshipReq); err != nil {
		errs = append(errs, err)
	}

	// 2.b remove all relationships to this role from its owner

	ownerRelReq := &pb.DeleteRelationshipsRequest{
		RelationshipFilter: &pb.RelationshipFilter{
			ResourceType: e.namespaced(roleOwner.Type),
			OptionalSubjectFilter: &pb.SubjectFilter{
				SubjectType:       e.namespaced(e.rbac.RoleResource.Name),
				OptionalSubjectId: roleResource.ID.String(),
			},
		},
	}

	if _, err := e.client.DeleteRelationships(ctx, ownerRelReq); err != nil {
		errs = append(errs, err)
	}

	for _, err := range errs {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			return err
		}
	}

	if err = e.store.CommitContext(dbCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		// At this point, spicedb changes have already been applied.
		// Attempting to rollback could result in failures that could result in the same situation.
		//
		// In this particular case, in the absence of spiceDB rollback, the expected
		// behavior in this case would be that the role only exists in the permissions-api
		// DB and not in spiceDB. This would result in the role being "orphaned"
		// (lack of ownership in spiceDB will prevent this role from being accessed or bind
		// by any user), and would need to be cleaned up manually.
		//
		// TODO: add spicedb rollback logic along with rollback failure scenarios.

		return err
	}

	return nil
}

// roleV2OwnerRelationship creates a relationships between a V2 role and its owner.
func (e *engine) roleV2OwnerRelationship(role types.Role, owner types.Resource) ([]*pb.RelationshipUpdate, error) {
	roleResource, err := e.NewResourceFromID(role.ID)
	if err != nil {
		return nil, err
	}

	roleResourceType := e.GetResourceType(e.rbac.RoleResource.Name)
	if roleResourceType == nil {
		return nil, nil
	}

	roleRef := resourceToSpiceDBRef(e.namespace, roleResource)
	ownerRef := resourceToSpiceDBRef(e.namespace, owner)

	// e.g., rolev2:super-admin#owner@tenant:tnntten-root
	ownerRel := &pb.RelationshipUpdate{
		Operation: pb.RelationshipUpdate_OPERATION_TOUCH,
		Relationship: &pb.Relationship{
			Resource: roleRef,
			Relation: iapl.RoleOwnerRelation,
			Subject: &pb.SubjectReference{
				Object: ownerRef,
			},
		},
	}

	// e.g., tenant:tnntten-root#member_role@rolev:super-admin
	memberRel := &pb.RelationshipUpdate{
		Operation: pb.RelationshipUpdate_OPERATION_TOUCH,
		Relationship: &pb.Relationship{
			Resource: ownerRef,
			Relation: iapl.RoleOwnerMemberRoleRelation,
			Subject: &pb.SubjectReference{
				Object: roleRef,
			},
		},
	}

	return []*pb.RelationshipUpdate{ownerRel, memberRel}, nil
}

// createRoleV2RelationshipUpdatesForAction creates permission relationship lines in role
// i.e., role:<role_name>#<action>_rel@<namespace>/<subjType>:*
func (e *engine) createRoleV2RelationshipUpdatesForAction(
	action string,
	roleRef *pb.ObjectReference,
	op pb.RelationshipUpdate_Operation,
) []*pb.RelationshipUpdate {
	rels := make([]*pb.RelationshipUpdate, len(e.rbac.RoleSubjectTypes))

	for i, subjType := range e.rbac.RoleSubjectTypes {
		rels[i] = &pb.RelationshipUpdate{
			Operation: op,
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
		}
	}

	return rels
}

// roleV2Relationships creates relationships between a V2 role and its permissions.
func (e *engine) roleV2Relationships(role types.Role) ([]*pb.RelationshipUpdate, error) {
	var rels []*pb.RelationshipUpdate

	roleResource, err := e.NewResourceFromID(role.ID)
	if err != nil {
		return nil, err
	}

	roleResourceType := e.GetResourceType(e.rbac.RoleResource.Name)
	if roleResourceType == nil {
		return rels, ErrRoleV2ResourceNotDefined
	}

	roleRef := resourceToSpiceDBRef(e.namespace, roleResource)

	for _, action := range role.Actions {
		rels = append(
			rels,
			e.createRoleV2RelationshipUpdatesForAction(
				action, roleRef,
				pb.RelationshipUpdate_OPERATION_TOUCH,
			)...,
		)
	}

	return rels, nil
}

func (e *engine) listRoleV2Actions(ctx context.Context, role types.Role) ([]string, error) {
	if len(e.rbac.RoleSubjectTypes) == 0 {
		return nil, nil
	}

	// there could be multiple subject types for a permission,
	// e.g.
	//   infratographer/rolev2:lb_viewer#loadbalancer_get_rel@infratographer/user:*
	//   infratographer/rolev2:lb_viewer#loadbalancer_get_rel@infratographer/client:*
	// here we only need one of them since the action is the only thing we care
	// about
	permRelationshipSubjType := e.namespaced(e.rbac.RoleSubjectTypes[0])

	rid := role.ID.String()
	filter := &pb.RelationshipFilter{
		ResourceType:       e.namespaced(e.rbac.RoleResource.Name),
		OptionalResourceId: rid,
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType:       permRelationshipSubjType,
			OptionalSubjectId: "*",
		},
	}

	relationships, err := e.readRelationships(ctx, filter)
	if err != nil {
		return nil, err
	}

	actions := make([]string, len(relationships))

	for i, rel := range relationships {
		actions[i] = relationToAction(rel.Relation)
	}

	return actions, nil
}

// AllActions list all available actions for a role
func (e *engine) AllActions() []string {
	rbv2, ok := e.schemaTypeMap[e.rbac.RoleBindingResource.Name]
	if !ok {
		return nil
	}

	actions := make([]string, len(rbv2.Actions))

	for i, action := range rbv2.Actions {
		actions[i] = action.Name
	}

	return actions
}
