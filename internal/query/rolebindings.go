package query

import (
	"context"
	"errors"
	"fmt"
	"sync"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/types"
)

func (e *engine) CreateRoleBinding(ctx context.Context, resource, roleResource types.Resource, subjects []types.RoleBindingSubject) (types.RoleBinding, error) {
	ctx, span := e.tracer.Start(
		ctx, "engine.BindRole",
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
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	role := types.Role{
		ID:   roleResource.ID,
		Name: dbrole.Name,
	}

	rbResourceType, ok := e.schemaTypeMap[e.rbac.RoleBindingResource.Name]
	if !ok {
		return types.RoleBinding{}, fmt.Errorf(
			"%w: invalid role-binding resource type: %s",
			ErrInvalidType, e.rbac.RoleBindingResource.Name,
		)
	}

	rb := newRoleBindingWithPrefix(rbResourceType.IDPrefix, role)
	roleRel := e.rolebindingRoleRelationship(role.ID.String(), rb.ID.String())

	grantRel, err := e.rolebindingGrantResourceRelationship(resource, rb.ID.String())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

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

		return types.RoleBinding{}, err
	}

	return rb, nil
}

func (e *engine) DeleteRoleBinding(ctx context.Context, rb, res types.Resource) error {
	ctx, span := e.tracer.Start(
		ctx, "engine.UnbindRole",
		trace.WithAttributes(
			attribute.Stringer("role_binding_id", rb.ID),
		),
	)
	defer span.End()

	// delete relationships from the role-binding
	if _, err := e.client.DeleteRelationships(ctx, &pb.DeleteRelationshipsRequest{
		RelationshipFilter: &pb.RelationshipFilter{
			ResourceType:       e.namespaced(e.rbac.RoleBindingResource.Name),
			OptionalResourceId: rb.ID.String(),
		},
	}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	// delete relationships to the role-binding
	if _, err := e.client.DeleteRelationships(ctx, &pb.DeleteRelationshipsRequest{
		RelationshipFilter: &pb.RelationshipFilter{
			ResourceType:     e.namespaced(res.Type),
			OptionalRelation: iapl.GrantRelationship,
			OptionalSubjectFilter: &pb.SubjectFilter{
				SubjectType:       e.namespaced(e.rbac.RoleBindingResource.Name),
				OptionalSubjectId: rb.ID.String(),
			},
		},
	}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

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
		return nil, err
	}

	// 2. fetch role-binding details for each grant
	bindings := make(chan types.RoleBinding, len(grantRel))
	errs := make(chan error, len(grantRel))
	wg := &sync.WaitGroup{}

	for _, rel := range grantRel {
		wg.Add(1)

		go func(grant *pb.Relationship) {
			defer wg.Done()

			rbRes, err := e.NewResourceFromIDString(grant.Subject.Object.ObjectId)
			if err != nil {
				errs <- err
				return
			}

			rb, err := e.fetchRoleBinding(ctx, rbRes)
			if err != nil {
				if errors.Is(err, ErrRoleBindingNotFound) {
					// print and record a warning message when there's a grant points
					// to a role-binding that not longer exists.
					err := fmt.Errorf("%w: dangling grant relationship: %s", err, grant.String())

					span.RecordError(err)
					e.logger.Warnf(err.Error())

					return
				}
				errs <- err

				return
			}

			if optionalRole != nil && rb.Role.ID.String() != optionalRole.ID.String() {
				return
			}

			if len(rb.Subjects) == 0 {
				return
			}

			bindings <- rb
		}(rel)
	}

	wg.Wait()
	close(errs)
	close(bindings)

	for err := range errs {
		if err != nil {
			return nil, err
		}
	}

	resp := make([]types.RoleBinding, 0, len(bindings))

	for rb := range bindings {
		resp = append(resp, rb)
	}

	return resp, nil
}

func (e *engine) UpdateRoleBinding(ctx context.Context, rb types.Resource, subjects []types.RoleBindingSubject) (types.RoleBinding, error) {
	ctx, span := e.tracer.Start(
		ctx, "engine.UpdateRoleBindings",
		trace.WithAttributes(
			attribute.Stringer("rolebinding_id", rb.ID),
		),
	)
	defer span.End()

	rolebinding, err := e.fetchRoleBinding(ctx, rb)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

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

	mkupdate := func(id string, op pb.RelationshipUpdate_Operation) (*pb.RelationshipUpdate, error) {
		subjRes, err := e.NewResourceFromIDString(id)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return nil, err
		}

		rel, err := e.rolebindingSubjectRelationship(subjRes, rb.ID.String())
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return nil, err
		}

		return &pb.RelationshipUpdate{Operation: op, Relationship: rel}, nil
	}

	for _, id := range add {
		update, err := mkupdate(id, pb.RelationshipUpdate_OPERATION_TOUCH)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return types.RoleBinding{}, err
		}

		updates[i] = update
		i++
	}

	for _, id := range remove {
		update, err := mkupdate(id, pb.RelationshipUpdate_OPERATION_DELETE)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return types.RoleBinding{}, err
		}

		updates[i] = update
		i++
	}

	_, err = e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{Updates: updates})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	rolebinding.Subjects = subjects

	return rolebinding, nil
}

func (e *engine) GetRoleBinding(ctx context.Context, rolebinding types.Resource) (types.RoleBinding, error) {
	ctx, span := e.tracer.Start(
		ctx, "engine.GetRoleBinding",
		trace.WithAttributes(attribute.Stringer("role_binding_id", rolebinding.ID)),
	)
	defer span.End()

	return e.fetchRoleBinding(ctx, rolebinding)
}

// isRoleBindable checks if a role is available for a resource. a role is not
// be available to a resource if it is owner is not associated with the resource
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

	requests := []*pb.DeleteRelationshipsRequest{}

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

	// 2. build a list of delete request for the subject and role relationship
	//    at the same time build a list of delete requests for all the grant
	//    relationships for all bindable resources
	for _, rb := range bindings {
		delSubjReq := &pb.DeleteRelationshipsRequest{
			RelationshipFilter: &pb.RelationshipFilter{
				ResourceType:       rb.Resource.ObjectType,
				OptionalResourceId: rb.Resource.ObjectId,
			},
		}

		requests = append(requests, delSubjReq)

		for _, res := range e.rolebindingV2Resources {
			delGrantReq := &pb.DeleteRelationshipsRequest{
				RelationshipFilter: &pb.RelationshipFilter{
					ResourceType:     e.namespaced(res.Name),
					OptionalRelation: iapl.GrantRelationship,
					OptionalSubjectFilter: &pb.SubjectFilter{
						SubjectType:       rb.Resource.ObjectType,
						OptionalSubjectId: rb.Resource.ObjectId,
					},
				},
			}

			requests = append(requests, delGrantReq)
		}
	}

	e.logger.Debugf("%d delete requests created", len(requests))

	// 3. delete all the relationships
	wg := &sync.WaitGroup{}
	errs := make(chan error, len(requests))

	for _, req := range requests {
		wg.Add(1)

		go func(req *pb.DeleteRelationshipsRequest) {
			defer wg.Done()

			if _, err := e.client.DeleteRelationships(ctx, req); err != nil {
				errs <- err
			}
		}(req)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *engine) fetchRoleBinding(ctx context.Context, roleBinding types.Resource) (types.RoleBinding, error) {
	ctx, span := e.tracer.Start(
		ctx, "engine.fetchRoleBinding",
		trace.WithAttributes(attribute.Stringer("role_binding_id", roleBinding.ID)),
	)
	defer span.End()

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

	rb := types.RoleBinding{
		ID:       roleBinding.ID,
		Subjects: make([]types.RoleBindingSubject, 0, len(rbRel)),
	}

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

// rolebindingSubjectRelationship is a helper function that creates a
// relationship between a role-binding and a subject.
func (e *engine) rolebindingSubjectRelationship(subj types.Resource, rbID string) (*pb.Relationship, error) {
	subjConf, ok := e.rolebindingV2SubjectsMap[subj.Type]
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

// NewResourceFromIDString creates a new resource from a string.
func (e *engine) NewResourceFromIDString(id string) (types.Resource, error) {
	subjID, err := gidx.Parse(id)
	if err != nil {
		return types.Resource{}, err
	}

	subject, err := e.NewResourceFromID(subjID)
	if err != nil {
		return types.Resource{}, err
	}

	return subject, nil
}

func newRoleBindingWithPrefix(prefix string, role types.Role) types.RoleBinding {
	rb := types.RoleBinding{
		ID:   gidx.MustNewID(prefix),
		Role: role,
	}

	return rb
}
