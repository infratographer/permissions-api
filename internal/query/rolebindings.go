package query

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/codes"

	"go.infratographer.com/permissions-api/internal/types"
)

// CreateRoleBinding creates all the necessary relationships for a role binding.
// role binding here establishes a three-way relationship between a role,
// a resource, and the subjects.
func (e *engine) CreateRoleBinding(ctx context.Context, roleResource types.Resource, resource types.Resource, subjects []string) (types.RoleBinding, error) {
	ctx, span := e.tracer.Start(ctx, "engine.CreateRoleBinding")
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

	rb := newRoleBindingWithPrefix(e.schemaTypeMap[e.rbac.RoleBindingResource].IDPrefix, role)
	rb.Subjects = make([]types.Resource, len(subjects))

	const ownerAndRole = 2
	updates := make([]*pb.RelationshipUpdate, len(subjects)+ownerAndRole)

	// 1. subject relationships
	for i, subj := range subjects {
		subj, err := e.NewResourceFromIDString(subj)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return types.RoleBinding{}, err
		}

		rb.Subjects[i] = subj

		rel, err := e.rolebindingSubjectRelationship(subj, rb.ID.String())
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return types.RoleBinding{}, err
		}

		updates[i] = &pb.RelationshipUpdate{
			Operation:    pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: rel,
		}
	}

	// 2. role relationship
	updates[len(updates)-2] = &pb.RelationshipUpdate{
		Operation:    pb.RelationshipUpdate_OPERATION_TOUCH,
		Relationship: e.rolebindingRoleRelationship(role, rb.ID.String()),
	}

	// 3. grant resource relationship
	grantRel, err := e.rolebindingGrantResourceRelationship(resource, rb.ID.String())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	updates[len(updates)-1] = &pb.RelationshipUpdate{
		Operation:    pb.RelationshipUpdate_OPERATION_TOUCH,
		Relationship: grantRel,
	}

	if _, err := e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{Updates: updates}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	return rb, nil
}

// GetRoleBinding fetches a role-binding object by its ID
func (e *engine) GetRoleBinding(ctx context.Context, resource, roleBinding types.Resource) (types.RoleBinding, error) {
	ctx, span := e.tracer.Start(ctx, "engine.GetRoleBinding")
	defer span.End()

	// 1. validate grant on resource, return not found if grant relationship does
	// not exist on the resource
	grantconf, ok := e.schemaRoleBindingsV2Map[resource.Type]
	if !ok {
		err := fmt.Errorf("%w: resource type: %s, resource ID: %s",
			ErrResourceDoesNotSupportRoleBindingV2,
			resource.Type, resource.ID.String(),
		)

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	grantRelFilter := &pb.RelationshipFilter{
		ResourceType:       e.namespaced(resource.Type),
		OptionalResourceId: resource.ID.String(),
		OptionalRelation:   grantconf.GrantRelationship,
	}

	grantRel, err := e.readRelationships(ctx, grantRelFilter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	if len(grantRel) < 1 {
		err := fmt.Errorf("%w: resource type: %s, resource ID: %s",
			ErrRoleBindingNotFound,
			resource.Type, resource.ID.String(),
		)

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return types.RoleBinding{}, err
	}

	// 2. get all role-bindings
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
		Subjects: make([]types.Resource, 0, len(rbRel)),
	}

	for _, rel := range rbRel {
		if rel.Relation == rolebindingSubjectRelation {
			subject, err := e.NewResourceFromIDString(rel.Subject.Object.ObjectId)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())

				return types.RoleBinding{}, err
			}

			rb.Subjects = append(rb.Subjects, subject)

			continue
		}

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

func (e *engine) ListRoleBindings(ctx context.Context, resource types.Resource) ([]types.RoleBinding, error) {
	ctx, span := e.tracer.Start(ctx, "engine.ListRoleBinding")
	defer span.End()

	// 1. get all grants from resource
	grantconf, ok := e.schemaRoleBindingsV2Map[resource.Type]
	if !ok {
		err := fmt.Errorf("%w: resource type: %s, resource ID: %s",
			ErrResourceDoesNotSupportRoleBindingV2,
			resource.Type, resource.ID.String(),
		)

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return nil, err
	}

	grantRelFilter := &pb.RelationshipFilter{
		ResourceType:       e.namespaced(resource.Type),
		OptionalResourceId: resource.ID.String(),
		OptionalRelation:   grantconf.GrantRelationship,
	}

	grantRel, err := e.readRelationships(ctx, grantRelFilter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return nil, err
	}

	// 2. get all rolebindings
	rolebindings := make(chan types.RoleBinding, len(grantRel))
	errs := make(chan error, len(grantRel))
	wg := &sync.WaitGroup{}

	getRoleBindingFn := func(rel *pb.Relationship) {
		defer wg.Done()

		rbRes, err := e.NewResourceFromIDString(rel.Subject.Object.ObjectId)
		if err != nil {
			errs <- err
			return
		}

		rb, err := e.GetRoleBinding(ctx, resource, rbRes)
		if err != nil {
			errs <- err
			return
		}

		// skip if there's a role-binding without a role
		if rb.Role.ID == "" {
			return
		}

		rolebindings <- rb
	}

	for _, rel := range grantRel {
		wg.Add(1)

		go getRoleBindingFn(rel)
	}

	wg.Wait()
	close(errs)
	close(rolebindings)

	for err := range errs {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return nil, err
		}
	}

	resp := make([]types.RoleBinding, 0, len(rolebindings))

	for rb := range rolebindings {
		resp = append(resp, rb)
	}

	return resp, nil
}

func (e *engine) UpdateRoleBinding(ctx context.Context, resource, roleBinding types.Resource, subjects []string) (types.RoleBinding, error) {
	return types.RoleBinding{}, nil
}

func (e *engine) DeleteRoleBinding(ctx context.Context, resource, roleBinding types.Resource) error {
	return nil
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

func (e *engine) rolebindingRoleRelationship(role types.Role, rbID string) *pb.Relationship {
	return &pb.Relationship{
		Resource: &pb.ObjectReference{
			ObjectType: e.namespaced(e.rbac.RoleBindingResource),
			ObjectId:   rbID,
		},
		Relation: rolebindingRoleRelation,
		Subject: &pb.SubjectReference{
			Object: &pb.ObjectReference{
				ObjectType: e.namespaced(e.rbac.RoleResource),
				ObjectId:   role.ID.String(),
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
