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

	"go.infratographer.com/permissions-api/internal/types"
)

// BindRole creates all the necessary relationships for a role binding.
// role binding here establishes a three-way relationship between a role,
// a resource, and the subjects.
// If a role-binding object already exists on the resource, new subjects
// relationships will be appended to the existing role-binding.
func (e *engine) BindRole(ctx context.Context, resource, roleResource types.Resource, subjects []types.RoleBindingSubject) (types.RoleBinding, error) {
	ctx, span := e.tracer.Start(
		ctx, "engine.BindRole",
		trace.WithAttributes(
			attribute.Stringer("role_id", roleResource.ID),
			attribute.Stringer("resource_id", resource.ID),
		),
	)
	defer span.End()

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

	rb, err := e.findOrCreateRoleBinding(ctx, resource, role)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	currSubjects := make(map[string]struct{}, len(rb.Subjects))
	updates := make([]*pb.RelationshipUpdate, 0, len(subjects))

	for _, subj := range rb.Subjects {
		currSubjects[subj.SubjectResource.ID.String()] = struct{}{}
	}

	for _, subj := range subjects {
		// skip if subject already in role-binding
		if _, ok := currSubjects[subj.SubjectResource.ID.String()]; ok {
			continue
		}

		rel, err := e.rolebindingSubjectRelationship(subj.SubjectResource, rb.ID.String())
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return types.RoleBinding{}, err
		}

		rb.Subjects = append(rb.Subjects, subj)

		updates = append(updates, &pb.RelationshipUpdate{
			Operation:    pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: rel,
		})
	}

	if len(updates) > 0 {
		if _, err := e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{Updates: updates}); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return types.RoleBinding{}, err
		}
	}

	return rb, nil
}

// UnbindRole removes subjects from a role-binding.
func (e *engine) UnbindRole(ctx context.Context, resource, roleResource types.Resource, subjects []types.RoleBindingSubject) error {
	ctx, span := e.tracer.Start(
		ctx, "engine.UnbindRole",
		trace.WithAttributes(
			attribute.Stringer("role_id", roleResource.ID),
			attribute.Stringer("resource_id", resource.ID),
		),
	)
	defer span.End()

	_, err := e.store.GetRoleByID(ctx, roleResource.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	bindings, err := e.ListRoleBindings(ctx, resource, &roleResource)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	if len(bindings) == 0 {
		// no bindings for role on the resoruce
		return nil
	} else if len(bindings) > 1 {
		// warn if there're multiple rolebindings for the role on the resource
		e.logger.Warnf("found multiple role-bindings for role: %s, resource: %s", roleResource.ID, resource.ID)
	}

	// remove subjects from role-binding
	for _, rb := range bindings {
		updates := make([]*pb.RelationshipUpdate, len(subjects))

		for i, subj := range subjects {
			rel, err := e.rolebindingSubjectRelationship(subj.SubjectResource, rb.ID.String())
			if err != nil {
				return err
			}

			updates[i] = &pb.RelationshipUpdate{
				Operation:    pb.RelationshipUpdate_OPERATION_DELETE,
				Relationship: rel,
			}
		}

		if _, err := e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{Updates: updates}); err != nil {
			return err
		}
	}

	return nil
}

// ListRoleBindings lists all role-bindings for a resource, an optional Role
// can be provided to filter the role-bindings.
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
	grantconf, ok := e.schemaRoleBindingsV2Map[resource.Type]
	if !ok {
		return nil, fmt.Errorf("%w: resource type: %s, resource ID: %s",
			ErrResourceDoesNotSupportRoleBindingV2,
			resource.Type, resource.ID.String(),
		)
	}

	listRbFilter := &pb.RelationshipFilter{
		ResourceType:       e.namespaced(resource.Type),
		OptionalResourceId: resource.ID.String(),
		OptionalRelation:   grantconf.GrantRelationship,
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType: e.namespaced(e.rbac.RoleBindingResource),
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

// deleteRoleBinding deletes all role-binding relationships with a given role.
func (e *engine) deleteRoleBinding(ctx context.Context, roleResource types.Resource) error {
	ctx, span := e.tracer.Start(
		ctx, "engine.deleteRoleBinding",
		trace.WithAttributes(
			attribute.Stringer("role_id", roleResource.ID),
		),
	)
	defer span.End()

	requests := []*pb.DeleteRelationshipsRequest{}

	// 1. find all the bindings for the role
	findBindingsFilter := &pb.RelationshipFilter{
		ResourceType:     e.namespaced(e.rbac.RoleBindingResource),
		OptionalRelation: rolebindingRoleRelation,
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType:       e.namespaced(e.rbac.RoleResource),
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

		for resName, grantconf := range e.schemaRoleBindingsV2Map {
			delGrantReq := &pb.DeleteRelationshipsRequest{
				RelationshipFilter: &pb.RelationshipFilter{
					ResourceType:     e.namespaced(resName),
					OptionalRelation: grantconf.GrantRelationship,
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

	// get all role-bindings relationships
	rbRelFilter := &pb.RelationshipFilter{
		ResourceType:       e.namespaced(e.rbac.RoleBindingResource),
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
		if rel.Relation == rolebindingSubjectRelation {
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

func (e *engine) findOrCreateRoleBinding(
	ctx context.Context, resource types.Resource, role types.Role,
) (types.RoleBinding, error) {
	ctx, span := e.tracer.Start(
		ctx, "engine.findOrCreateRoleBinding",
		trace.WithAttributes(
			attribute.Stringer("role_id", role.ID),
			attribute.Stringer("resource_id", resource.ID),
		),
	)
	defer span.End()

	roleResource := &types.Resource{
		Type: e.rbac.RoleResource,
		ID:   role.ID,
	}

	bindings, err := e.ListRoleBindings(ctx, resource, roleResource)
	if err != nil {
		return types.RoleBinding{}, err
	}

	var roleBinding types.RoleBinding

	switch len(bindings) {
	case 0:
		// create new binding if no matching role-binding found
		rbResourceType, ok := e.schemaTypeMap[e.rbac.RoleBindingResource]
		if !ok {
			return types.RoleBinding{}, fmt.Errorf(
				"%w: invalid role-binding resource type: %s",
				ErrInvalidType, e.rbac.RoleBindingResource,
			)
		}

		roleBinding = newRoleBindingWithPrefix(rbResourceType.IDPrefix, role)
		roleRel := e.rolebindingRoleRelationship(role.ID.String(), roleBinding.ID.String())

		grantRel, err := e.rolebindingGrantResourceRelationship(resource, roleBinding.ID.String())
		if err != nil {
			return types.RoleBinding{}, err
		}

		_, err = e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{
			Updates: []*pb.RelationshipUpdate{
				{
					Operation:    pb.RelationshipUpdate_OPERATION_TOUCH,
					Relationship: roleRel,
				},
				{
					Operation:    pb.RelationshipUpdate_OPERATION_TOUCH,
					Relationship: grantRel,
				},
			},
		})
		if err != nil {
			return types.RoleBinding{}, err
		}

	case 1:
		roleBinding = bindings[0]
	default:
		roleBinding = bindings[0]

		// if there's more than one matching bindings,
		// return the first role-binding one and print a warning message
		msg := fmt.Sprintf("found multiple role-bindings for role: %s, resource: %s", role.ID, resource.ID)
		e.logger.Warn(msg)
	}

	return roleBinding, nil
}

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
			ObjectType: e.namespaced(e.rbac.RoleBindingResource),
			ObjectId:   rbID,
		},
		Relation: rolebindingSubjectRelation,
		Subject:  relationshipSubject,
	}

	return relationship, nil
}

func (e *engine) rolebindingRoleRelationship(roleID, rbID string) *pb.Relationship {
	return &pb.Relationship{
		Resource: &pb.ObjectReference{
			ObjectType: e.namespaced(e.rbac.RoleBindingResource),
			ObjectId:   rbID,
		},
		Relation: rolebindingRoleRelation,
		Subject: &pb.SubjectReference{
			Object: &pb.ObjectReference{
				ObjectType: e.namespaced(e.rbac.RoleResource),
				ObjectId:   roleID,
			},
		},
	}
}

func (e *engine) rolebindingGrantResourceRelationship(resource types.Resource, rbID string) (*pb.Relationship, error) {
	grantconf, ok := e.schemaRoleBindingsV2Map[resource.Type]
	if !ok {
		return nil, fmt.Errorf("%w: resource type: %s, resource ID: %s",
			ErrResourceDoesNotSupportRoleBindingV2,
			resource.Type, resource.ID.String(),
		)
	}

	rel := &pb.Relationship{
		Resource: &pb.ObjectReference{
			ObjectType: e.namespaced(resource.Type),
			ObjectId:   resource.ID.String(),
		},
		Relation: grantconf.GrantRelationship,
		Subject: &pb.SubjectReference{
			Object: &pb.ObjectReference{
				ObjectType: e.namespaced(e.rbac.RoleBindingResource),
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
