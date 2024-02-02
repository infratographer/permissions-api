package query

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/types"
)

var roleSubjectRelation = "subject"

func (e *engine) getTypeForResource(res types.Resource) (types.ResourceType, error) {
	for _, resType := range e.schema {
		if res.Type == resType.Name {
			return resType, nil
		}
	}

	return types.ResourceType{}, ErrInvalidType
}

func (e *engine) validateRelationship(rel types.Relationship) error {
	subjType, err := e.getTypeForResource(rel.Subject)
	if err != nil {
		return err
	}

	resType, err := e.getTypeForResource(rel.Resource)
	if err != nil {
		return err
	}

	e.logger.Debugw("validation relationship", "sub", subjType.Name, "rel", rel.Relation, "res", resType.Name)

	for _, typeRel := range resType.Relationships {
		// If we find a relation with a name and type that matches our relationship,
		// return
		if rel.Relation == typeRel.Relation {
			for _, t := range typeRel.Types {
				if subjType.Name == t.Name {
					return nil
				}
			}
		}
	}

	// No matching relationship was found, so we should return an error
	return ErrInvalidRelationship
}

func resourceToSpiceDBRef(namespace string, r types.Resource) *pb.ObjectReference {
	return &pb.ObjectReference{
		ObjectType: namespace + "/" + r.Type,
		ObjectId:   r.ID.String(),
	}
}

// SubjectHasPermission checks if the given subject can do the given action on the given resource
func (e *engine) SubjectHasPermission(ctx context.Context, subject types.Resource, action string, resource types.Resource) error {
	ctx, span := e.tracer.Start(
		ctx,
		"SubjectHasPermission",
		trace.WithAttributes(
			attribute.Stringer(
				"permissions.actor",
				subject.ID,
			),
			attribute.String(
				"permissions.action",
				action,
			),
			attribute.Stringer(
				"permissions.resource",
				resource.ID,
			),
		),
	)

	defer span.End()

	consistency, consName := e.determineConsistency(ctx, resource)
	span.SetAttributes(
		attribute.String(
			"permissions.consistency",
			consName,
		),
	)

	req := &pb.CheckPermissionRequest{
		Consistency: consistency,
		Resource:    resourceToSpiceDBRef(e.namespace, resource),
		Permission:  action,
		Subject: &pb.SubjectReference{
			Object: resourceToSpiceDBRef(e.namespace, subject),
		},
	}

	err := e.checkPermission(ctx, req)

	switch {
	case err == nil:
		span.SetAttributes(
			attribute.String(
				"permissions.outcome",
				outcomeAllowed,
			),
		)
	case errors.Is(err, ErrActionNotAssigned):
		span.SetAttributes(
			attribute.String(
				"permissions.outcome",
				outcomeDenied,
			),
		)
	default:
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}

// AssignSubjectRole assigns the given role to the given subject.
func (e *engine) AssignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) error {
	request := &pb.WriteRelationshipsRequest{
		Updates: []*pb.RelationshipUpdate{
			e.subjectRoleRelCreate(subject, role),
		},
	}

	if _, err := e.client.WriteRelationships(ctx, request); err != nil {
		return err
	}

	return nil
}

// UnassignSubjectRole removes the given role from the given subject.
func (e *engine) UnassignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) error {
	request := &pb.DeleteRelationshipsRequest{
		RelationshipFilter: e.subjectRoleRelDelete(subject, role),
	}

	if _, err := e.client.DeleteRelationships(ctx, request); err != nil {
		return err
	}

	return nil
}

// ListAssignments returns the assigned subjects for a given role.
func (e *engine) ListAssignments(ctx context.Context, role types.Role) ([]types.Resource, error) {
	roleType := e.namespace + "/role"
	filter := &pb.RelationshipFilter{
		ResourceType:       roleType,
		OptionalResourceId: role.ID.String(),
		OptionalRelation:   roleSubjectRelation,
	}

	relationships, err := e.readRelationships(ctx, filter)
	if err != nil {
		return nil, err
	}

	out := make([]types.Resource, len(relationships))

	for i, rel := range relationships {
		id, err := gidx.Parse(rel.Subject.Object.ObjectId)
		if err != nil {
			return nil, err
		}

		res, err := e.NewResourceFromID(id)
		if err != nil {
			return nil, err
		}

		out[i] = res
	}

	return out, nil
}

func (e *engine) subjectRoleRelCreate(subject types.Resource, role types.Role) *pb.RelationshipUpdate {
	roleResource := types.Resource{
		Type: "role",
		ID:   role.ID,
	}

	return &pb.RelationshipUpdate{
		Operation: pb.RelationshipUpdate_OPERATION_CREATE,
		Relationship: &pb.Relationship{
			Resource: resourceToSpiceDBRef(e.namespace, roleResource),
			Relation: roleSubjectRelation,
			Subject: &pb.SubjectReference{
				Object: resourceToSpiceDBRef(e.namespace, subject),
			},
		},
	}
}

func (e *engine) subjectRoleRelDelete(subject types.Resource, role types.Role) *pb.RelationshipFilter {
	roleResource := types.Resource{
		Type: "role",
		ID:   role.ID,
	}

	return &pb.RelationshipFilter{
		ResourceType:       e.namespace + "/" + roleResource.Type,
		OptionalResourceId: roleResource.ID.String(),
		OptionalRelation:   roleSubjectRelation,
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType:       e.namespace + "/" + subject.Type,
			OptionalSubjectId: subject.ID.String(),
		},
	}
}

func (e *engine) checkPermission(ctx context.Context, req *pb.CheckPermissionRequest) error {
	resp, err := e.client.CheckPermission(ctx, req)
	if err != nil {
		return err
	}

	if resp.Permissionship == pb.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION {
		return nil
	}

	return ErrActionNotAssigned
}

// CreateRelationships atomically creates the given relationships in SpiceDB.
func (e *engine) CreateRelationships(ctx context.Context, rels []types.Relationship) error {
	ctx, span := e.tracer.Start(ctx, "engine.CreateRelationships", trace.WithAttributes(attribute.Int("relationships", len(rels))))

	defer span.End()

	for _, rel := range rels {
		err := e.validateRelationship(rel)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return err
		}
	}

	relUpdates := e.relationshipsToUpdates(rels)

	request := &pb.WriteRelationshipsRequest{
		Updates: relUpdates,
	}

	resp, err := e.client.WriteRelationships(ctx, request)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	e.updateRelationshipZedTokens(ctx, rels, resp.WrittenAt.Token)

	return nil
}

// CreateRole creates a role scoped to the given resource with the given actions.
func (e *engine) CreateRole(ctx context.Context, actor, res types.Resource, roleName string, actions []string) (types.Role, error) {
	ctx, span := e.tracer.Start(ctx, "engine.CreateRole")

	defer span.End()

	roleName = strings.TrimSpace(roleName)

	role := newRole(roleName, actions)
	roleRels := e.roleRelationships(role, res)

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
		return types.Role{}, nil
	}

	dbRole, err := e.store.CreateRole(dbCtx, actor.ID, role.ID, roleName, res.ID)
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

// diff determines which entities needs to be added and removed.
// If no new entity are provided it is assumed no changes are requested.
func diff(current, incoming []string) ([]string, []string) {
	if len(incoming) == 0 {
		return nil, nil
	}

	curr := make(map[string]struct{}, len(current))
	in := make(map[string]struct{}, len(incoming))

	var add, rem []string

	for _, entity := range current {
		curr[entity] = struct{}{}
	}

	for _, action := range incoming {
		in[action] = struct{}{}

		// If the new action is not in the old actions, then we need to add the action.
		if _, ok := curr[action]; !ok {
			add = append(add, action)
		}
	}

	for _, action := range current {
		// If the old action is not in the new actions, then we need to remove it.
		if _, ok := in[action]; !ok {
			rem = append(rem, action)
		}
	}

	return add, rem
}

// UpdateRole allows for updating an existing role with a new name and new actions.
// If new name is empty, no change is made.
// If new actions is an empty slice, no change is made.
func (e *engine) UpdateRole(ctx context.Context, actor, roleResource types.Resource, newName string, newActions []string) (types.Role, error) {
	ctx, span := e.tracer.Start(ctx, "engine.UpdateRole")

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

	role, err := e.GetRole(dbCtx, roleResource)
	if err != nil {
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.Role{}, err
	}

	newName = strings.TrimSpace(newName)

	if newName == "" {
		newName = role.Name
	}

	addActions, remActions := diff(role.Actions, newActions)

	// If no changes, return existing role with no changes.
	if newName == role.Name && len(addActions) == 0 && len(remActions) == 0 {
		return role, nil
	}

	resource, err := e.NewResourceFromID(role.ResourceID)
	if err != nil {
		return types.Role{}, err
	}

	dbRole, err := e.store.UpdateRole(dbCtx, actor.ID, role.ID, newName)
	if err != nil {
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.Role{}, err
	}

	// If a change in actions, apply changes to spicedb.
	if len(addActions) != 0 || len(remActions) != 0 {
		roleRels := e.roleResourceRelationshipsTouchDelete(roleResource, resource, addActions, remActions)

		request := &pb.WriteRelationshipsRequest{Updates: roleRels}

		if _, err := e.client.WriteRelationships(ctx, request); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			return types.Role{}, err
		}

		role.Actions = newActions
	}

	if err := e.store.CommitContext(dbCtx); err != nil {
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

	return role, nil
}

func logRollbackErr(logger *zap.SugaredLogger, err error, args ...interface{}) {
	if err != nil {
		logger.With(args...).Error("error while rolling back", zap.Error(err))
	}
}

func actionToRelation(action string) string {
	return action + "_rel"
}

func relationToAction(relation string) string {
	action, _, found := strings.Cut(relation, "_rel")

	if !found {
		panic("unexpected relation on role")
	}

	return action
}

func (e *engine) roleRelationships(role types.Role, resource types.Resource) []*pb.RelationshipUpdate {
	var rels []*pb.RelationshipUpdate

	roleResource, err := e.NewResourceFromID(role.ID)
	if err != nil {
		panic(err)
	}

	resourceRef := resourceToSpiceDBRef(e.namespace, resource)
	roleRef := resourceToSpiceDBRef(e.namespace, roleResource)

	for _, action := range role.Actions {
		rels = append(rels, &pb.RelationshipUpdate{
			Operation: pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: &pb.Relationship{
				Resource: resourceRef,
				Relation: actionToRelation(action),
				Subject: &pb.SubjectReference{
					Object:           roleRef,
					OptionalRelation: roleSubjectRelation,
				},
			},
		})
	}

	return rels
}

func (e *engine) roleResourceRelationshipsTouchDelete(roleResource, resource types.Resource, touchActions, deleteActions []string) []*pb.RelationshipUpdate {
	var rels []*pb.RelationshipUpdate

	resourceRef := resourceToSpiceDBRef(e.namespace, resource)
	roleRef := resourceToSpiceDBRef(e.namespace, roleResource)

	for _, action := range touchActions {
		rels = append(rels, &pb.RelationshipUpdate{
			Operation: pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: &pb.Relationship{
				Resource: resourceRef,
				Relation: actionToRelation(action),
				Subject: &pb.SubjectReference{
					Object:           roleRef,
					OptionalRelation: roleSubjectRelation,
				},
			},
		})
	}

	for _, action := range deleteActions {
		rels = append(rels, &pb.RelationshipUpdate{
			Operation: pb.RelationshipUpdate_OPERATION_DELETE,
			Relationship: &pb.Relationship{
				Resource: resourceRef,
				Relation: actionToRelation(action),
				Subject: &pb.SubjectReference{
					Object:           roleRef,
					OptionalRelation: roleSubjectRelation,
				},
			},
		})
	}

	return rels
}

func (e *engine) relationshipsToUpdates(rels []types.Relationship) []*pb.RelationshipUpdate {
	relUpdates := make([]*pb.RelationshipUpdate, len(rels))

	for i, rel := range rels {
		subjRef := resourceToSpiceDBRef(e.namespace, rel.Subject)
		resRef := resourceToSpiceDBRef(e.namespace, rel.Resource)

		relUpdates[i] = &pb.RelationshipUpdate{
			Operation: pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: &pb.Relationship{
				Resource: resRef,
				Relation: rel.Relation,
				Subject: &pb.SubjectReference{
					Object: subjRef,
				},
			},
		}
	}

	return relUpdates
}

func (e *engine) readRelationships(ctx context.Context, filter *pb.RelationshipFilter) ([]*pb.Relationship, error) {
	req := pb.ReadRelationshipsRequest{
		Consistency: &pb.Consistency{
			Requirement: &pb.Consistency_FullyConsistent{
				FullyConsistent: true,
			},
		},
	}

	req.RelationshipFilter = filter

	r, err := e.client.ReadRelationships(ctx, &req)
	if err != nil {
		return nil, err
	}

	var (
		responses []*pb.Relationship
		done      bool
	)

	for !done {
		rel, err := r.Recv()
		switch err {
		case nil:
			responses = append(responses, rel.Relationship)
		case io.EOF:
			done = true
		default:
			return nil, err
		}
	}

	return responses, nil
}

// DeleteRelationships removes the specified relationships.
// If any relationships fails to be deleted, all completed deletions are re-created.
func (e *engine) DeleteRelationships(ctx context.Context, relationships ...types.Relationship) error {
	ctx, span := e.tracer.Start(ctx, "engine.DeleteRelationships", trace.WithAttributes(attribute.Int("relationships", len(relationships))))

	defer span.End()

	var errors []error

	span.AddEvent("validating relationships")

	for i, relationship := range relationships {
		err := e.validateRelationship(relationship)
		if err != nil {
			err = fmt.Errorf("%w: invalid relationship %d", err, i)

			span.RecordError(err)

			errors = append(errors, err)
		}
	}

	if len(errors) != 0 {
		span.SetStatus(codes.Error, "invalid relationships")

		return multierr.Combine(errors...)
	}

	errors = []error{}

	var (
		complete []types.Relationship
		dErr     error
		cErr     error
	)

	span.AddEvent("deleting relationships")

	for i, relationship := range relationships {
		resType := e.namespace + "/" + relationship.Resource.Type
		subjType := e.namespace + "/" + relationship.Subject.Type

		filter := &pb.RelationshipFilter{
			ResourceType:       resType,
			OptionalResourceId: relationship.Resource.ID.String(),
			OptionalRelation:   relationship.Relation,
			OptionalSubjectFilter: &pb.SubjectFilter{
				SubjectType:       subjType,
				OptionalSubjectId: relationship.Subject.ID.String(),
			},
		}

		if dErr = e.deleteRelationships(ctx, filter); dErr != nil {
			e.logger.Errorf("%w: failed to delete relationship %d reverting %d completed deletes", dErr, i, len(complete))

			err := fmt.Errorf("%w: failed to delete relationship %d", dErr, i)

			span.RecordError(err)

			errors = append(errors, err)

			break
		}

		complete = append(complete, relationship)
	}

	if len(errors) != 0 {
		span.SetStatus(codes.Error, "error occurred deleting relationships")

		if len(complete) != 0 {
			span.AddEvent("recreating deleted relationships")

			if cErr = e.CreateRelationships(ctx, complete); cErr != nil {
				e.logger.Error("%w: failed to revert %d deleted relationships", cErr, len(complete))

				err := fmt.Errorf("%w: failed to revert deleted relationships", cErr)

				span.RecordError(err)

				errors = append(errors, err)
			}
		}

		return multierr.Combine(errors...)
	}

	return nil
}

// DeleteResourceRelationships deletes all relationships originating from the given resource.
func (e *engine) DeleteResourceRelationships(ctx context.Context, resource types.Resource) error {
	resType := e.namespace + "/" + resource.Type

	filter := &pb.RelationshipFilter{
		ResourceType:       resType,
		OptionalResourceId: resource.ID.String(),
	}

	return e.deleteRelationships(ctx, filter)
}

func (e *engine) deleteRelationships(ctx context.Context, filter *pb.RelationshipFilter) error {
	request := &pb.DeleteRelationshipsRequest{
		RelationshipFilter: filter,
	}

	if _, err := e.client.DeleteRelationships(ctx, request); err != nil {
		return err
	}

	return nil
}

func relationshipsToRoles(rels []*pb.Relationship) []types.Role {
	var roleIDs []gidx.PrefixedID

	roleMap := make(map[gidx.PrefixedID]*types.Role)

	for _, rel := range rels {
		roleIDStr := rel.Subject.Object.ObjectId

		roleID, err := gidx.Parse(roleIDStr)
		if err != nil {
			panic(err)
		}

		action := relationToAction(rel.Relation)

		_, ok := roleMap[roleID]
		if !ok {
			roleIDs = append(roleIDs, roleID)
			role := types.Role{
				ID: roleID,
			}
			roleMap[roleID] = &role
		}

		roleMap[roleID].Actions = append(roleMap[roleID].Actions, action)
	}

	out := make([]types.Role, len(roleIDs))
	for i, roleID := range roleIDs {
		out[i] = *roleMap[roleID]
	}

	return out
}

func (e *engine) relationshipsToNonRoles(rels []*pb.Relationship) ([]types.Relationship, error) {
	var out []types.Relationship

	for _, rel := range rels {
		// skip relationships for v1 roles, and wildcard relationships for v2 roles
		if rel.Subject.Object.ObjectType == e.namespace+"/role" || rel.Subject.Object.ObjectId == "*" {
			continue
		}

		resID, err := gidx.Parse(rel.Resource.ObjectId)
		if err != nil {
			return nil, err
		}

		res, err := e.NewResourceFromID(resID)
		if err != nil {
			return nil, err
		}

		subjID, err := gidx.Parse(rel.Subject.Object.ObjectId)
		if err != nil {
			return nil, err
		}

		subj, err := e.NewResourceFromID(subjID)
		if err != nil {
			return nil, err
		}

		item := types.Relationship{
			Resource: res,
			Relation: rel.Relation,
			Subject:  subj,
		}

		out = append(out, item)
	}

	return out, nil
}

// ListRelationshipsFrom returns all non-role relationships bound to a given resource.
func (e *engine) ListRelationshipsFrom(ctx context.Context, resource types.Resource) ([]types.Relationship, error) {
	resType := e.namespace + "/" + resource.Type

	filter := &pb.RelationshipFilter{
		ResourceType:       resType,
		OptionalResourceId: resource.ID.String(),
	}

	relationships, err := e.readRelationships(ctx, filter)
	if err != nil {
		return nil, err
	}

	return e.relationshipsToNonRoles(relationships)
}

// ListRelationshipsTo returns all non-role relationships destined for a given resource.
func (e *engine) ListRelationshipsTo(ctx context.Context, resource types.Resource) ([]types.Relationship, error) {
	relTypes, ok := e.schemaSubjectRelationMap[resource.Type]
	if !ok {
		return nil, ErrInvalidType
	}

	var relationships []*pb.Relationship

	for _, types := range relTypes {
		for _, relType := range types {
			rels, err := e.readRelationships(ctx, &pb.RelationshipFilter{
				ResourceType: e.namespace + "/" + relType,
				OptionalSubjectFilter: &pb.SubjectFilter{
					SubjectType:       e.namespace + "/" + resource.Type,
					OptionalSubjectId: resource.ID.String(),
				},
			})
			if err != nil {
				return nil, err
			}

			relationships = append(relationships, rels...)
		}
	}

	return e.relationshipsToNonRoles(relationships)
}

// ListRoles returns all roles bound to a given resource.
func (e *engine) ListRoles(ctx context.Context, resource types.Resource) ([]types.Role, error) {
	dbRoles, err := e.store.ListResourceRoles(ctx, resource.ID)
	if err != nil {
		return nil, err
	}

	dbRolesv1 := make([]storage.Role, 0, len(dbRoles))

	for _, dbRole := range dbRoles {
		res, err := e.NewResourceFromID(dbRole.ID)
		if err != nil {
			return nil, err
		}

		if res.Type == e.rbac.RoleResource {
			continue
		}

		dbRolesv1 = append(dbRolesv1, dbRole)
	}

	resType := e.namespace + "/" + resource.Type
	roleType := e.namespace + "/role"

	filter := &pb.RelationshipFilter{
		ResourceType:       resType,
		OptionalResourceId: resource.ID.String(),
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType: roleType,
			OptionalRelation: &pb.SubjectFilter_RelationFilter{
				Relation: roleSubjectRelation,
			},
		},
	}

	relationships, err := e.readRelationships(ctx, filter)
	if err != nil {
		return nil, err
	}

	spicedbRoles := relationshipsToRoles(relationships)

	rolesByID := make(map[gidx.PrefixedID]types.Role, len(spicedbRoles))

	for _, role := range spicedbRoles {
		rolesByID[role.ID] = role
	}

	out := make([]types.Role, len(dbRolesv1))

	for i, dbRole := range dbRolesv1 {
		spicedbRole := rolesByID[dbRole.ID]

		out[i] = types.Role{
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

	return out, nil
}

// listRoleResourceActions returns all resources and action relations for the provided resource type to the provided role.
// Note: The actions returned by this function are the spicedb relationship action.
func (e *engine) listRoleResourceActions(ctx context.Context, role types.Resource, resTypeName string) (map[types.Resource][]string, error) {
	resType := e.namespace + "/" + resTypeName
	roleType := e.namespace + "/role"

	filter := &pb.RelationshipFilter{
		ResourceType: resType,
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType:       roleType,
			OptionalSubjectId: role.ID.String(),
			OptionalRelation: &pb.SubjectFilter_RelationFilter{
				Relation: roleSubjectRelation,
			},
		},
	}

	relationships, err := e.readRelationships(ctx, filter)
	if err != nil {
		return nil, err
	}

	resourceIDActions := make(map[gidx.PrefixedID][]string)

	for _, rel := range relationships {
		resourceID, err := gidx.Parse(rel.Resource.ObjectId)
		if err != nil {
			return nil, err
		}

		resourceIDActions[resourceID] = append(resourceIDActions[resourceID], rel.Relation)
	}

	resourceActions := make(map[types.Resource][]string, len(resourceIDActions))

	for resID, actions := range resourceIDActions {
		res, err := e.NewResourceFromID(resID)
		if err != nil {
			return nil, err
		}

		resourceActions[res] = actions
	}

	return resourceActions, nil
}

// GetRole gets the role with it's actions.
func (e *engine) GetRole(ctx context.Context, roleResource types.Resource) (types.Role, error) {
	var (
		resActions map[types.Resource][]string
		err        error
	)

	for _, resType := range e.schemaRoleables {
		resActions, err = e.listRoleResourceActions(ctx, roleResource, resType.Name)
		if err != nil {
			return types.Role{}, err
		}

		// roles are only ever created for a single resource, so we can break after the first one is found.
		if len(resActions) != 0 {
			break
		}
	}

	if len(resActions) > 1 {
		return types.Role{}, ErrRoleHasTooManyResources
	}

	// returns the first resources actions.
	for _, actions := range resActions {
		for i, action := range actions {
			actions[i] = relationToAction(action)
		}

		dbRole, err := e.store.GetRoleByID(ctx, roleResource.ID)
		if err != nil && !errors.Is(err, storage.ErrNoRoleFound) {
			e.logger.Error("error while getting role", zap.Error(err))
		}

		return types.Role{
			ID:      roleResource.ID,
			Name:    dbRole.Name,
			Actions: actions,

			ResourceID: dbRole.ResourceID,
			CreatedBy:  dbRole.CreatedBy,
			UpdatedBy:  dbRole.UpdatedBy,
			CreatedAt:  dbRole.CreatedAt,
			UpdatedAt:  dbRole.UpdatedAt,
		}, nil
	}

	return types.Role{}, ErrRoleNotFound
}

// GetRoleResource gets the role's assigned resource.
func (e *engine) GetRoleResource(ctx context.Context, roleResource types.Resource) (types.Resource, error) {
	var (
		resActions map[types.Resource][]string
		err        error
	)

	for _, resType := range e.schemaRoleables {
		resActions, err = e.listRoleResourceActions(ctx, roleResource, resType.Name)
		if err != nil {
			return types.Resource{}, err
		}

		// roles are only ever created for a single resource, so we can break after the first one is found.
		if len(resActions) != 0 {
			break
		}
	}

	if len(resActions) > 1 {
		return types.Resource{}, ErrRoleHasTooManyResources
	}

	// returns the first resources actions.
	for resource := range resActions {
		return resource, nil
	}

	return types.Resource{}, ErrRoleNotFound
}

// DeleteRole removes all role actions from the assigned resource.
func (e *engine) DeleteRole(ctx context.Context, roleResource types.Resource) error {
	ctx, span := e.tracer.Start(ctx, "engine.DeleteRole")

	defer span.End()

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
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

	var resActions map[types.Resource][]string

	for _, resType := range e.schemaRoleables {
		resActions, err = e.listRoleResourceActions(ctx, roleResource, resType.Name)
		if err != nil {
			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			return err
		}

		// roles are only ever created for a single resource, so we can break after the first one is found.
		if len(resActions) != 0 {
			break
		}
	}

	roleType := e.namespace + "/role"

	var filters []*pb.RelationshipFilter

	roleSubjectFilter := &pb.SubjectFilter{
		SubjectType:       roleType,
		OptionalSubjectId: roleResource.ID.String(),
		OptionalRelation: &pb.SubjectFilter_RelationFilter{
			Relation: roleSubjectRelation,
		},
	}

	for resource, relActions := range resActions {
		for _, relAction := range relActions {
			filters = append(filters, &pb.RelationshipFilter{
				ResourceType:          e.namespace + "/" + resource.Type,
				OptionalResourceId:    resource.ID.String(),
				OptionalRelation:      relAction,
				OptionalSubjectFilter: roleSubjectFilter,
			})
		}
	}

	_, err = e.store.DeleteRole(dbCtx, roleResource.ID)
	if err != nil {
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	for _, filter := range filters {
		if err = e.deleteRelationships(ctx, filter); err != nil {
			err = fmt.Errorf("failed to delete role action %s: %w", filter.OptionalResourceId, err)

			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			// At this point, some spicedb changes may have already been applied.
			// Attempting to rollback could result in failures that could result in the same situation.
			//
			// TODO: add spicedb rollback logic along with rollback failure scenarios.

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

// NewResourceFromID returns a new resource struct from a given id
func (e *engine) NewResourceFromID(id gidx.PrefixedID) (types.Resource, error) {
	prefix := id.Prefix()

	rType, ok := e.schemaPrefixMap[prefix]
	if !ok {
		return types.Resource{}, ErrInvalidNamespace
	}

	out := types.Resource{
		Type: rType.Name,
		ID:   id,
	}

	return out, nil
}

// GetResourceType returns the resource type by name
func (e *engine) GetResourceType(name string) *types.ResourceType {
	rType, ok := e.schemaTypeMap[name]
	if !ok {
		return nil
	}

	return &rType
}
