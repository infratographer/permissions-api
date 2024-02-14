package query

import (
	"context"
	"fmt"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.infratographer.com/x/gidx"

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
			return types.RoleBinding{}, err
		}

		rb.Subjects[i] = subj

		rel, err := e.rolebindingSubjectRelationship(subj, rb.ID.String())
		if err != nil {
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
		return types.RoleBinding{}, err
	}

	updates[len(updates)-1] = &pb.RelationshipUpdate{
		Operation:    pb.RelationshipUpdate_OPERATION_TOUCH,
		Relationship: grantRel,
	}

	if _, err := e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{Updates: updates}); err != nil {
		return types.RoleBinding{}, err
	}

	return rb, nil
}

func (e *engine) ListRoleBindings(ctx context.Context, role types.Resource) ([]types.RoleBinding, error) {
	return nil, nil
}

func (e *engine) GetRoleBinding(ctx context.Context, roleBinding types.Resource) (types.RoleBinding, error) {
	return types.RoleBinding{}, nil
}

func (e *engine) UpdateRoleBinding(ctx context.Context, roleBinding, role types.Resource, subjects []string) (types.RoleBinding, error) {
	return types.RoleBinding{}, nil
}

func (e *engine) DeleteRoleBinding(ctx context.Context, roleBinding types.Resource) error {
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
