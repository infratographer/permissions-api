package query

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/types"
)

// V2 Role and Role Bindings

func (e *engine) namespaced(name string) string {
	return e.namespace + "/" + name
}

func (e *engine) CreateRoleV2(ctx context.Context, actor, owner types.Resource, roleName string, actions []string) (types.Role, error) {
	ctx, span := e.tracer.Start(ctx, "engine.CreateRoleV2")

	defer span.End()

	roleName = strings.TrimSpace(roleName)

	role := newRoleWithPrefix(e.schemaTypeMap[e.rbac.RoleResource.Name].IDPrefix, roleName, actions)
	roleRels := e.roleV2Relationships(role)
	roleRels = append(roleRels, e.roleV2OwnerRelationship(role, owner)...)

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

	roleIDs := []string{}

	for {
		lookup, err := lookupClient.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}

			break
		}

		subj := lookup.GetSubject()
		roleIDs = append(roleIDs, subj.SubjectObjectId)
	}

	roles := make([]types.Role, len(roleIDs))
	errsChan := make(chan error, len(roleIDs))
	wg := &sync.WaitGroup{}

	for i, id := range roleIDs {
		wg.Add(1)

		roleRes, err := e.NewResourceFromIDString(id)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			e.logger.Error(err.Error())

			wg.Done()

			continue
		}

		go func(ctx context.Context, res types.Resource, i int) {
			defer wg.Done()

			role, err := e.GetRoleV2(ctx, res)
			if err != nil {
				errsChan <- err
				return
			}

			roles[i] = role
		}(ctx, roleRes, i)
	}

	wg.Wait()
	close(errsChan)

	for err := range errsChan {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}

	return roles, nil
}

func (e *engine) GetRoleV2(ctx context.Context, role types.Resource) (types.Role, error) {
	const ReadRolesErrBufLen = 2

	var (
		actions []string
		dbrole  storage.Role
		err     error
		errs    = make(chan error, ReadRolesErrBufLen)
		wg      = &sync.WaitGroup{}
	)

	ctx, span := e.tracer.Start(
		ctx,
		"engine.GetRoleV2",
		trace.WithAttributes(attribute.Stringer("role", role.ID)),
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
	wg.Add(1)

	go func() {
		defer wg.Done()

		spicedbctx, span := e.tracer.Start(ctx, "listRoleV2Actions")
		defer span.End()

		actions, err = e.listRoleV2Actions(spicedbctx, types.Role{ID: role.ID})
		if err != nil {
			errs <- err
			return
		}
	}()

	// 2. Get role info (name, created_by, etc.) from permissions API DB
	wg.Add(1)

	go func() {
		defer wg.Done()

		apidbctx, span := e.tracer.Start(ctx, "getRoleFromPermissionAPI")
		defer span.End()

		dbrole, err = e.store.GetRoleByID(apidbctx, role.ID)
		if err != nil {
			errs <- err
			return
		}
	}()

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return types.Role{}, err
		}
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

	newName = strings.TrimSpace(newName)

	if newName == "" {
		newName = role.Name
	}

	addActions, rmActions := diff(role.Actions, newActions)

	// If no changes, return existing role
	if newName == role.Name && len(addActions) == 0 && len(rmActions) == 0 {
		if err = e.store.CommitContext(dbCtx); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		return role, nil
	}

	// 1. update role in permission-api DB
	dbRole, err := e.store.UpdateRole(dbCtx, actor.ID, role.ID, newName)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.Role{}, err
	}

	// 2. update permissions relationships in spice db
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

	// 2.c write updates to spicedb
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

		// At this point, spicedb changes have already been applied.
		// Attempting to rollback could result in failures that could result in the same situation.
		//
		// TODO: add spicedb rollback logic along with rollback failure scenarios.

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

	dbRole, err := e.store.GetRoleByID(ctx, roleResource.ID)
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

	// 1. delete role from permission-api DB
	if _, err = e.store.DeleteRole(dbCtx, roleResource.ID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	// 2. delete role relationships from spice db
	const deleteErrsBufferSize = 2

	wg := &sync.WaitGroup{}
	errs := make(chan error, deleteErrsBufferSize)

	// 2.a remove all relationships from this role
	wg.Add(1)

	go func() {
		defer wg.Done()

		delRoleRelationshipReq := &pb.DeleteRelationshipsRequest{
			RelationshipFilter: &pb.RelationshipFilter{
				ResourceType:       e.namespaced(e.rbac.RoleResource.Name),
				OptionalResourceId: roleResource.ID.String(),
			},
		}

		if _, err := e.client.DeleteRelationships(ctx, delRoleRelationshipReq); err != nil {
			errs <- err
		}
	}()

	// 2.b remove all relationships to this role from its owner
	wg.Add(1)

	go func() {
		defer wg.Done()

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
			errs <- err
		}
	}()

	// 2.c remove all role relationships in role bindings associated with this role
	wg.Add(1)

	go func() {
		defer wg.Done()

		if err := e.deleteRoleBindingsForRole(ctx, roleResource); err != nil {
			errs <- err
		}
	}()

	wg.Wait()
	close(errs)

	for err := range errs {
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
		// TODO: add spicedb rollback logic along with rollback failure scenarios.

		return err
	}

	return nil
}

// roleV2OwnerRelationship creates a relationships between a V2 role and its owner.
func (e *engine) roleV2OwnerRelationship(role types.Role, owner types.Resource) []*pb.RelationshipUpdate {
	roleResource, err := e.NewResourceFromID(role.ID)
	if err != nil {
		panic(err)
	}

	roleResourceType := e.GetResourceType(e.rbac.RoleResource.Name)
	if roleResourceType == nil {
		return nil
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

	return []*pb.RelationshipUpdate{ownerRel, memberRel}
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
func (e *engine) roleV2Relationships(role types.Role) []*pb.RelationshipUpdate {
	var rels []*pb.RelationshipUpdate

	roleResource, err := e.NewResourceFromID(role.ID)
	if err != nil {
		panic(err)
	}

	roleResourceType := e.GetResourceType(e.rbac.RoleResource.Name)
	if roleResourceType == nil {
		return rels
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

	return rels
}

func (e *engine) listRoleV2Actions(ctx context.Context, role types.Role) ([]string, error) {
	if len(e.rbac.RoleSubjectTypes) == 0 {
		return nil, nil
	}

	// there could be multiple subjects for a permission,
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
