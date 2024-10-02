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
		trace.WithAttributes(attribute.Stringer("rolebinding_id", roleBinding.ID)),
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
		err := ErrRoleBindingHasNoRelationships

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	rb.SubjectIDs = make([]gidx.PrefixedID, 0, len(rbRel))

	for _, rel := range rbRel {
		switch {
		// process subject relationships
		case rel.Relation == iapl.RolebindingSubjectRelation:
			subjID, err := gidx.Parse(rel.Subject.Object.ObjectId)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())

				return types.RoleBinding{}, err
			}

			rb.SubjectIDs = append(rb.SubjectIDs, subjID)

		// process role relationships
		default:
			rb.RoleID, err = gidx.Parse(rel.Subject.Object.ObjectId)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())

				return types.RoleBinding{}, err
			}
		}
	}

	return rb, nil
}

func (e *engine) CreateRoleBinding(
	ctx context.Context,
	actor, resource, roleResource types.Resource,
	manager string,
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

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, nil
	}

	rbResourceType := e.schemaTypeMap[e.rbac.RoleBindingResource.Name]

	rbid, err := gidx.NewID(rbResourceType.IDPrefix)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.RoleBinding{}, err
	}

	rb, err := e.store.CreateRoleBinding(dbCtx, actor.ID, rbid, resource.ID, manager)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.RoleBinding{}, err
	}

	rb.RoleID = dbrole.ID

	roleRel := e.rolebindingRoleRelationship(dbrole.ID.String(), rb.ID.String())

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
	rb.SubjectIDs = make([]gidx.PrefixedID, len(subjects))

	for i, subj := range subjects {
		rel, err := e.rolebindingSubjectRelationship(subj.SubjectResource, rb.ID.String())
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			return types.RoleBinding{}, err
		}

		rb.SubjectIDs[i] = subj.SubjectResource.ID
		subjUpdates[i] = &pb.RelationshipUpdate{
			Operation:    pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: rel,
		}
	}

	updates = append(updates, subjUpdates...)

	if err := e.applyUpdates(dbCtx, updates); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return types.RoleBinding{}, err
	}

	if err := e.store.CommitContext(dbCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))
		logRollbackErr(e.logger, e.rollbackUpdates(ctx, updates))

		return types.RoleBinding{}, err
	}

	return rb, nil
}

func (e *engine) DeleteRoleBinding(ctx context.Context, rb types.Resource) error {
	ctx, span := e.tracer.Start(
		ctx, "engine.DeleteRoleBinding",
		trace.WithAttributes(
			attribute.Stringer("rolebinding_id", rb.ID),
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
	if err := e.applyUpdates(dbCtx, updates); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

		return err
	}

	if err := e.store.DeleteRoleBinding(dbCtx, rb.ID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))
		logRollbackErr(e.logger, e.rollbackUpdates(ctx, updates))

		return err
	}

	if err := e.store.CommitContext(dbCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))
		logRollbackErr(e.logger, e.rollbackUpdates(ctx, updates))

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

	return e.listRoleBindings(ctx, resource, optionalRole, nil)
}

func (e *engine) ListManagerRoleBindings(ctx context.Context, manager string, resource types.Resource, optionalRole *types.Resource) ([]types.RoleBinding, error) {
	ctx, span := e.tracer.Start(
		ctx, "engine.ListManagerRoleBinding",
		trace.WithAttributes(
			attribute.Stringer("resource_id", resource.ID),
			attribute.String("manager", manager),
		),
	)
	defer span.End()

	e.logger.Debugf("listing manager %s role-bindings for resource: %s, optionalRole: %v", manager, resource.ID, optionalRole)

	return e.listRoleBindings(ctx, resource, optionalRole, &manager)
}

func (e *engine) listRoleBindings(ctx context.Context, resource types.Resource, optionalRole *types.Resource, optionalManager *string) ([]types.RoleBinding, error) {
	span := trace.SpanFromContext(ctx)

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
				span.SetStatus(codes.Error, err.Error())
				e.logger.Warnf(err.Error())
			}

			errs = append(errs, err)

			continue
		}

		if optionalManager != nil && rb.Manager != *optionalManager {
			continue
		}

		if optionalRole != nil && rb.RoleID != optionalRole.ID {
			continue
		}

		if len(rb.SubjectIDs) == 0 {
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
	current := make([]string, len(rolebinding.SubjectIDs))
	incoming := make([]string, len(subjects))
	newSubjectIDs := make([]gidx.PrefixedID, len(subjects))

	for i, subj := range rolebinding.SubjectIDs {
		current[i] = subj.String()
	}

	for i, subj := range subjects {
		incoming[i] = subj.SubjectResource.ID.String()
		newSubjectIDs[i] = subj.SubjectResource.ID
	}

	add, remove := diff(current, incoming, true)

	// return if there are no changes
	if (len(add) + len(remove)) == 0 {
		return rolebinding, nil
	}

	// 2. create relationship updates
	updates := make([]*pb.RelationshipUpdate, 0, len(add)+len(remove))

	for _, id := range add {
		update, err := e.rolebindingRelationshipUpdateForSubject(id, rb.ID.String(), pb.RelationshipUpdate_OPERATION_TOUCH)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			return types.RoleBinding{}, err
		}

		updates = append(updates, update)
	}

	for _, id := range remove {
		update, err := e.rolebindingRelationshipUpdateForSubject(id, rb.ID.String(), pb.RelationshipUpdate_OPERATION_DELETE)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			return types.RoleBinding{}, err
		}

		updates = append(updates, update)
	}

	if err := e.applyUpdates(dbCtx, updates); err != nil {
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
		logRollbackErr(e.logger, e.rollbackUpdates(ctx, updates))
	}

	if err := e.store.CommitContext(dbCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))
		logRollbackErr(e.logger, e.rollbackUpdates(ctx, updates))

		return types.RoleBinding{}, err
	}

	rolebinding.SubjectIDs = newSubjectIDs
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
