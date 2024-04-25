package query

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/types"
)

func (e *engine) GetRoleBinding(ctx context.Context, roleBinding types.Resource) (types.RoleBinding, error) {
	ctx, span := e.tracer.Start(
		ctx, "engine.GetRoleBinding",
		trace.WithAttributes(attribute.Stringer("role_binding_id", roleBinding.ID)),
	)
	defer span.End()

	rb, err := e.store.GetRoleBindingByID(ctx, roleBinding.ID)
	if err != nil {
		if errors.Is(err, storage.ErrRoleBindingNotFound) {
			err = fmt.Errorf("%w: role-binding: %s", ErrRoleBindingNotFound, err)
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	// gather all relationships from this role-binding
	rbRelFilter := &pb.RelationshipFilter{
		ResourceType:       e.namespaced(e.rbac.RoleBindingResource.Name),
		OptionalResourceId: roleBinding.ID.String(),
	}

	rbRel, err := e.readRelationships(ctx, rbRelFilter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	if len(rbRel) < 1 {
		err := fmt.Errorf("%w: role binding: %s", ErrRoleBindingNotFound, roleBinding.ID.String())

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	rb.Subjects = make([]types.RoleBindingSubject, 0, len(rbRel))

	for _, rel := range rbRel {
		// process subject relationships
		if rel.Relation == iapl.RolebindingSubjectRelation {
			subjectRes, err := e.NewResourceFromIDString(rel.Subject.Object.ObjectId)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())

				return types.RoleBinding{}, err
			}

			rb.Subjects = append(rb.Subjects, types.RoleBindingSubject{SubjectResource: subjectRes})

			continue
		}

		// process role relationships
		roleID, err := gidx.Parse(rel.Subject.Object.ObjectId)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return types.RoleBinding{}, err
		}

		dbRole, err := e.store.GetRoleByID(ctx, roleID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return types.RoleBinding{}, err
		}

		rb.Role = types.Role{
			ID:   roleID,
			Name: dbRole.Name,
		}
	}

	return rb, nil
}

func (e *engine) CreateRoleBinding(
	ctx context.Context,
	actor, resource, roleResource types.Resource,
	subjects []types.RoleBindingSubject,
) (types.RoleBinding, error) {
	ctx, span := e.tracer.Start(
		ctx, "engine.CreateRoleBinding",
		trace.WithAttributes(
			attribute.Stringer("role_id", roleResource.ID),
			attribute.Stringer("resource_id", resource.ID),
		),
	)
	defer span.End()

	if err := e.isRoleBindable(ctx, roleResource, resource); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	dbrole, err := e.store.GetRoleByID(ctx, roleResource.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNoRoleFound) {
			err = fmt.Errorf("%w: role %s", ErrRoleNotFound, roleResource.ID)
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	role := types.Role{
		ID:   roleResource.ID,
		Name: dbrole.Name,
	}

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
		return types.RoleBinding{}, nil
	}

	rbResourceType, ok := e.schemaTypeMap[e.rbac.RoleBindingResource.Name]
	if !ok {
		return types.RoleBinding{}, fmt.Errorf(
			"%w: invalid role-binding resource type: %s",
			ErrInvalidType, e.rbac.RoleBindingResource.Name,
		)
	}

	rbid, err := gidx.NewID(rbResourceType.IDPrefix)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.RoleBinding{}, err
	}

	rb, err := e.store.CreateRoleBinding(dbCtx, actor.ID, rbid, resource.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.RoleBinding{}, err
	}

	rb.Role = role

	roleRel := e.rolebindingRoleRelationship(role.ID.String(), rb.ID.String())

	grantRel, err := e.rolebindingGrantResourceRelationship(resource, rb.ID.String())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.RoleBinding{}, err
	}

	updates := []*pb.RelationshipUpdate{
		{
			Operation:    pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: roleRel,
		},
		{
			Operation:    pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: grantRel,
		},
	}

	subjUpdates := make([]*pb.RelationshipUpdate, len(subjects))
	rb.Subjects = make([]types.RoleBindingSubject, len(subjects))

	for i, subj := range subjects {
		rel, err := e.rolebindingSubjectRelationship(subj.SubjectResource, rb.ID.String())
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			return types.RoleBinding{}, err
		}

		rb.Subjects[i] = subj
		subjUpdates[i] = &pb.RelationshipUpdate{
			Operation:    pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: rel,
		}
	}

	if _, err := e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{
		Updates: append(updates, subjUpdates...),
	}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.RoleBinding{}, err
	}

	if err := e.store.CommitContext(dbCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))
		logRollbackErr(e.logger, e.rollbackRoleBindingUpdates(ctx, updates))

		return types.RoleBinding{}, err
	}

	return rb, nil
}

func (e *engine) DeleteRoleBinding(ctx context.Context, rb types.Resource) error {
	ctx, span := e.tracer.Start(
		ctx, "engine.DeleteRoleBinding",
		trace.WithAttributes(
			attribute.Stringer("role_binding_id", rb.ID),
		),
	)
	defer span.End()

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	if err := e.store.LockRoleBindingForUpdate(dbCtx, rb.ID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	rbFromDB, err := e.store.GetRoleBindingByID(dbCtx, rb.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	res, err := e.NewResourceFromID(rbFromDB.ResourceID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	// gather all relationships from the role-binding resource
	fromRels, err := e.readRelationships(ctx, &pb.RelationshipFilter{
		ResourceType:       e.namespaced(e.rbac.RoleBindingResource.Name),
		OptionalResourceId: rb.ID.String(),
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	// gather relationships to the role-binding
	toRels, err := e.readRelationships(ctx, &pb.RelationshipFilter{
		ResourceType:     e.namespaced(res.Type),
		OptionalRelation: iapl.GrantRelationship,
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType:       e.namespaced(e.rbac.RoleBindingResource.Name),
			OptionalSubjectId: rb.ID.String(),
		},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	// create a list of delete updates for these relationships
	updates := make([]*pb.RelationshipUpdate, len(fromRels)+len(toRels))

	for i, rel := range append(fromRels, toRels...) {
		updates[i] = &pb.RelationshipUpdate{
			Operation:    pb.RelationshipUpdate_OPERATION_DELETE,
			Relationship: rel,
		}
	}

	// apply changes
	if _, err := e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{Updates: updates}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	if err := e.store.DeleteRoleBinding(dbCtx, rb.ID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))
		logRollbackErr(e.logger, e.rollbackRoleBindingUpdates(ctx, updates))

		return err
	}

	if err := e.store.CommitContext(dbCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))
		logRollbackErr(e.logger, e.rollbackRoleBindingUpdates(ctx, updates))

		return err
	}

	return nil
}

func (e *engine) ListRoleBindings(ctx context.Context, resource types.Resource, optionalRole *types.Resource) ([]types.RoleBinding, error) {
	ctx, span := e.tracer.Start(
		ctx, "engine.ListRoleBinding",
		trace.WithAttributes(
			attribute.Stringer("resource_id", resource.ID),
		),
	)
	defer span.End()

	e.logger.Debugf("listing role-bindings for resource: %s, optionalRole: %v", resource.ID, optionalRole)

	// 1. list all grants on the resource
	listRbFilter := &pb.RelationshipFilter{
		ResourceType:       e.namespaced(resource.Type),
		OptionalResourceId: resource.ID.String(),
		OptionalRelation:   iapl.GrantRelationship,
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType: e.namespaced(e.rbac.RoleBindingResource.Name),
		},
	}

	grantRel, err := e.readRelationships(ctx, listRbFilter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return nil, err
	}

	// 2. fetch role-binding details for each grant
	bindings := make([]types.RoleBinding, 0, len(grantRel))
	errs := make([]error, 0, len(grantRel))

	for _, rel := range grantRel {
		rbRes, err := e.NewResourceFromIDString(rel.Subject.Object.ObjectId)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		rb, err := e.GetRoleBinding(ctx, rbRes)
		if err != nil {
			if errors.Is(err, ErrRoleBindingNotFound) {
				// print and record a warning message when there's a grant points
				// to a role-binding that not longer exists.
				//
				// this should not happen in normal circumstances, but it's possible
				// if some role-binding relationships are deleted directly through
				// spiceDB
				err := fmt.Errorf("%w: dangling grant relationship: %s", err, rel.String())

				span.RecordError(err)
				e.logger.Warnf(err.Error())
			}

			errs = append(errs, err)

			continue
		}

		if optionalRole != nil && rb.Role.ID.String() != optionalRole.ID.String() {
			continue
		}

		if len(rb.Subjects) == 0 {
			continue
		}

		bindings = append(bindings, rb)
	}

	for _, err := range errs {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return nil, err
		}
	}

	return bindings, nil
}

func (e *engine) UpdateRoleBinding(ctx context.Context, actor, rb types.Resource, subjects []types.RoleBindingSubject) (types.RoleBinding, error) {
	ctx, span := e.tracer.Start(
		ctx, "engine.UpdateRoleBindings",
		trace.WithAttributes(
			attribute.Stringer("rolebinding_id", rb.ID),
		),
	)
	defer span.End()

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	if err := e.store.LockRoleBindingForUpdate(dbCtx, rb.ID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.RoleBinding{}, err
	}

	rolebinding, err := e.GetRoleBinding(dbCtx, rb)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.RoleBinding{}, err
	}

	// 1. find the subjects to add or remove
	current := make([]string, len(rolebinding.Subjects))
	incoming := make([]string, len(subjects))

	for i, subj := range rolebinding.Subjects {
		current[i] = subj.SubjectResource.ID.String()
	}

	for i, subj := range subjects {
		incoming[i] = subj.SubjectResource.ID.String()
	}

	add, remove := diff(current, incoming)

	// return if there are no changes
	if (len(add) + len(remove)) == 0 {
		return rolebinding, nil
	}

	// 2. create relationship updates
	updates := make([]*pb.RelationshipUpdate, len(add)+len(remove))
	i := 0

	for _, id := range add {
		update, err := e.rolebindingRelationshipUpdateForSubject(id, rb.ID.String(), pb.RelationshipUpdate_OPERATION_TOUCH)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			return types.RoleBinding{}, err
		}

		updates[i] = update
		i++
	}

	for _, id := range remove {
		update, err := e.rolebindingRelationshipUpdateForSubject(id, rb.ID.String(), pb.RelationshipUpdate_OPERATION_DELETE)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			return types.RoleBinding{}, err
		}

		updates[i] = update
		i++
	}

	_, err = e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{Updates: updates})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.RoleBinding{}, err
	}

	// 3. update the role-binding in the database to record latest `updatedBy` and `updatedAt`
	rbFromDB, err := e.store.UpdateRoleBinding(dbCtx, actor.ID, rb.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))
		logRollbackErr(e.logger, e.rollbackRoleBindingUpdates(ctx, updates))
	}

	if err := e.store.CommitContext(dbCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))
		logRollbackErr(e.logger, e.rollbackRoleBindingUpdates(ctx, updates))

		return types.RoleBinding{}, err
	}

	rolebinding.Subjects = subjects
	rolebinding.UpdatedAt = rbFromDB.UpdatedAt
	rolebinding.UpdatedBy = rbFromDB.UpdatedBy

	return rolebinding, nil
}

func (e *engine) GetRoleBindingResource(ctx context.Context, rb types.Resource) (types.Resource, error) {
	rbFromDB, err := e.store.GetRoleBindingByID(ctx, rb.ID)
	if err != nil {
		if errors.Is(err, storage.ErrRoleBindingNotFound) {
			err = fmt.Errorf("%w: %s", ErrRoleBindingNotFound, err)
		}

		return types.Resource{}, err
	}

	return e.NewResourceFromID(rbFromDB.ResourceID)
}

// isRoleBindable checks if a role is available for a resource. a role is not
// available to a resource if its owner is not associated with the resource
// in any way.
func (e *engine) isRoleBindable(ctx context.Context, role, res types.Resource) error {
	req := &pb.CheckPermissionRequest{
		Resource: &pb.ObjectReference{
			ObjectType: e.namespaced(res.Type),
			ObjectId:   res.ID.String(),
		},
		Subject: &pb.SubjectReference{
			Object: &pb.ObjectReference{
				ObjectType: e.namespaced(e.rbac.RoleResource.Name),
				ObjectId:   role.ID.String(),
			},
		},
		Permission: iapl.AvailableRolesList,
		Consistency: &pb.Consistency{
			Requirement: &pb.Consistency_FullyConsistent{FullyConsistent: true},
		},
	}

	err := e.checkPermission(ctx, req)

	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrActionNotAssigned):
		return fmt.Errorf("%w: role: %s is not available for resource: %s", ErrRoleNotFound, role.ID, res.ID)
	default:
		return err
	}
}

// deleteRoleBindingsForRole deletes all role-binding relationships with a given role.
func (e *engine) deleteRoleBindingsForRole(ctx context.Context, roleResource types.Resource) error {
	ctx, span := e.tracer.Start(
		ctx, "engine.deleteRoleBindingsForRole",
		trace.WithAttributes(
			attribute.Stringer("role_id", roleResource.ID),
		),
	)
	defer span.End()

	// 1. find all the bindings for the role
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

	if len(bindings) == 0 {
		return nil
	}

	rbIDs := make([]gidx.PrefixedID, len(bindings))

	for i, rel := range bindings {
		id, err := gidx.Parse(rel.Resource.ObjectId)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return err
		}

		rbIDs[i] = id
	}

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	if err := e.store.BatchLockRoleBindingForUpdate(dbCtx, rbIDs); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	// 2. Gather all the relationships to be deleted

	// 2.1 build a list of requests to get all the subject, role and grant
	//     relationships for all bindable resources
	relFilters := []*pb.RelationshipFilter{}

	for _, rb := range bindings {
		relFilters = append(relFilters, &pb.RelationshipFilter{
			ResourceType:       rb.Resource.ObjectType,
			OptionalResourceId: rb.Resource.ObjectId,
		},
		)

		for _, res := range e.rbacV2ResourceTypes {
			relFilters = append(relFilters, &pb.RelationshipFilter{
				ResourceType:     e.namespaced(res.Name),
				OptionalRelation: iapl.GrantRelationship,
				OptionalSubjectFilter: &pb.SubjectFilter{
					SubjectType:       rb.Resource.ObjectType,
					OptionalSubjectId: rb.Resource.ObjectId,
				},
			},
			)
		}
	}

	// 2.2 read all the relationships
	rels := []*pb.Relationship{}

	for _, filter := range relFilters {
		r, err := e.readRelationships(ctx, filter)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			return err
		}

		rels = append(rels, r...)
	}

	// 2.3 create delete requests
	updates := make([]*pb.RelationshipUpdate, len(rels))

	for i, rel := range rels {
		updates[i] = &pb.RelationshipUpdate{
			Operation:    pb.RelationshipUpdate_OPERATION_DELETE,
			Relationship: rel,
		}
	}

	e.logger.Debugf("%d relationships will be deleted", len(updates))

	// 3.1 delete all records in permissions-api DB
	if err := e.store.BatchDeleteRoleBindings(dbCtx, rbIDs); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	// 3.2 delete all the relationships
	if _, err := e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{Updates: updates}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	if err := e.store.CommitContext(dbCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))
		logRollbackErr(e.logger, e.rollbackRoleBindingUpdates(ctx, updates))

		return err
	}

	return nil
}

// rolebindingSubjectRelationship is a helper function that creates a
// relationship between a role-binding and a subject.
func (e *engine) rolebindingSubjectRelationship(subj types.Resource, rbID string) (*pb.Relationship, error) {
	subjConf, ok := e.rolebindingSubjectsMap[subj.Type]
	if !ok {
		return nil, fmt.Errorf(
			"%w: subject: %s, subject type: %s", ErrInvalidRoleBindingSubjectType,
			subj.ID, subj.Type,
		)
	}

	relationshipSubject := &pb.SubjectReference{
		Object: &pb.ObjectReference{
			ObjectType: e.namespaced(subjConf.Name),
			ObjectId:   subj.ID.String(),
		},
	}

	// for grants like "group#member"
	if subjConf.SubjectRelation != "" {
		relationshipSubject.OptionalRelation = subjConf.SubjectRelation
	}

	relationship := &pb.Relationship{
		Resource: &pb.ObjectReference{
			ObjectType: e.namespaced(e.rbac.RoleBindingResource.Name),
			ObjectId:   rbID,
		},
		Relation: iapl.RolebindingSubjectRelation,
		Subject:  relationshipSubject,
	}

	return relationship, nil
}

// rolebindingRoleRelationship is a helper function that creates a relationship
// between a role-binding and a role.
func (e *engine) rolebindingRoleRelationship(roleID, rbID string) *pb.Relationship {
	return &pb.Relationship{
		Resource: &pb.ObjectReference{
			ObjectType: e.namespaced(e.rbac.RoleBindingResource.Name),
			ObjectId:   rbID,
		},
		Relation: iapl.RolebindingRoleRelation,
		Subject: &pb.SubjectReference{
			Object: &pb.ObjectReference{
				ObjectType: e.namespaced(e.rbac.RoleResource.Name),
				ObjectId:   roleID,
			},
		},
	}
}

// rolebindingGrantResourceRelationship is a helper function that creates the
// `grant` relationship from a resource to a role-binding.
func (e *engine) rolebindingGrantResourceRelationship(resource types.Resource, rbID string) (*pb.Relationship, error) {
	rel := &pb.Relationship{
		Resource: &pb.ObjectReference{
			ObjectType: e.namespaced(resource.Type),
			ObjectId:   resource.ID.String(),
		},
		Relation: iapl.GrantRelationship,
		Subject: &pb.SubjectReference{
			Object: &pb.ObjectReference{
				ObjectType: e.namespaced(e.rbac.RoleBindingResource.Name),
				ObjectId:   rbID,
			},
		},
	}

	return rel, nil
}

// rolebindingRelationshipUpdateForSubject is a helper function that creates a
// relationship update that adds the given subject to a role-binding update
// request
func (e *engine) rolebindingRelationshipUpdateForSubject(
	subjID, rolebindingID string, op pb.RelationshipUpdate_Operation,
) (*pb.RelationshipUpdate, error) {
	subjRes, err := e.NewResourceFromIDString(subjID)
	if err != nil {
		return nil, err
	}

	rel, err := e.rolebindingSubjectRelationship(subjRes, rolebindingID)
	if err != nil {
		return nil, err
	}

	return &pb.RelationshipUpdate{Operation: op, Relationship: rel}, nil
}

// rollbackRoleBindingUpdates is a helper function that rolls back a list of
// relationship updates on spiceDB.
func (e *engine) rollbackRoleBindingUpdates(ctx context.Context, updates []*pb.RelationshipUpdate) error {
	updatesLen := len(updates)
	rollbacks := make([]*pb.RelationshipUpdate, 0, updatesLen)

	for i := range updates {
		// reversed order
		u := updates[updatesLen-i-1]

		if u == nil {
			continue
		}

		var op pb.RelationshipUpdate_Operation

		switch u.Operation {
		case pb.RelationshipUpdate_OPERATION_CREATE:
			fallthrough
		case pb.RelationshipUpdate_OPERATION_TOUCH:
			op = pb.RelationshipUpdate_OPERATION_DELETE
		case pb.RelationshipUpdate_OPERATION_DELETE:
			op = pb.RelationshipUpdate_OPERATION_TOUCH
		default:
			continue
		}

		rollbacks = append(rollbacks, &pb.RelationshipUpdate{
			Operation:    op,
			Relationship: u.Relationship,
		})
	}

	_, err := e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{Updates: rollbacks})

	return err
}
