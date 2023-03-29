package query

import (
	"context"
	"errors"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.infratographer.com/permissions-api/internal/types"
	"go.infratographer.com/x/urnx"
)

var roleSubjectRelation = "subject"

var (
	roleTemplateAdmin = types.RoleTemplate{
		Actions: []string{
			"loadbalancer_create",
			"loadbalancer_update",
			"loadbalancer_list",
			"loadbalancer_get",
			"loadbalancer_delete",
		},
	}

	roleTemplates = []types.RoleTemplate{
		roleTemplateAdmin,
	}

	errorInvalidNamespace = errors.New("invalid namespace")
)

func resourceToSpiceDBRef(namespace string, r types.Resource) *pb.ObjectReference {
	return &pb.ObjectReference{
		ObjectType: namespace + "/" + r.Type,
		ObjectId:   r.ID.String(),
	}
}

// SubjectHasPermission checks if the given subject can do the given action on the given resource
func (e *Engine) SubjectHasPermission(ctx context.Context, subject types.Resource, action string, resource types.Resource, queryToken string) error {
	req := &pb.CheckPermissionRequest{
		Resource:   resourceToSpiceDBRef(e.namespace, resource),
		Permission: action,
		Subject: &pb.SubjectReference{
			Object: resourceToSpiceDBRef(e.namespace, subject),
		},
	}

	return e.checkPermission(ctx, req, queryToken)
}

// AssignSubjectRole assigns the given role to the given subject
func (e *Engine) AssignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) (string, error) {
	request := &pb.WriteRelationshipsRequest{
		Updates: []*pb.RelationshipUpdate{
			e.subjectRoleRel(subject, role),
		},
	}
	r, err := e.client.WriteRelationships(ctx, request)

	if err != nil {
		return "", err
	}

	return r.WrittenAt.GetToken(), nil
}

func (e *Engine) subjectRoleRel(subject types.Resource, role types.Role) *pb.RelationshipUpdate {
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

func (e *Engine) checkPermission(ctx context.Context, req *pb.CheckPermissionRequest, queryToken string) error {
	if queryToken != "" {
		req.Consistency = &pb.Consistency{Requirement: &pb.Consistency_AtLeastAsFresh{AtLeastAsFresh: &pb.ZedToken{Token: queryToken}}}
	}

	resp, err := e.client.CheckPermission(ctx, req)
	if err != nil {
		return err
	}

	if resp.Permissionship == pb.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION {
		return nil
	}

	return ErrActionNotAssigned
}

// CreateBuiltInRoles generates the builtin roles for a resource
func (e *Engine) CreateBuiltInRoles(ctx context.Context, context types.Resource) ([]types.Role, string, error) {
	var (
		roles    []types.Role
		roleRels []*pb.RelationshipUpdate
	)

	for _, t := range roleTemplates {
		role := newRoleFromTemplate(t)
		roles = append(roles, role)
		roleRels = append(roleRels, e.roleRelationships(role, context)...)
	}

	request := &pb.WriteRelationshipsRequest{Updates: roleRels}

	r, err := e.client.WriteRelationships(ctx, request)
	if err != nil {
		return nil, "", err
	}

	return roles, r.WrittenAt.GetToken(), nil
}

func actionToRelation(action string) string {
	return action + "_rel"
}

func (e *Engine) roleRelationships(role types.Role, resource types.Resource) []*pb.RelationshipUpdate {
	var rels []*pb.RelationshipUpdate

	roleResource := types.Resource{
		Type: "role",
		ID:   role.ID,
	}

	resourceRef := resourceToSpiceDBRef(e.namespace, resource)
	roleRef := resourceToSpiceDBRef(e.namespace, roleResource)

	for _, action := range role.Actions {
		rels = append(rels, &pb.RelationshipUpdate{
			Operation: pb.RelationshipUpdate_OPERATION_CREATE,
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

func GetResourceTypes() []types.ResourceType {
	return []types.ResourceType{
		{
			Name: "tenant",
			Relationships: []types.ResourceRelationship{
				{
					Name:     "tenant",
					Type:     "tenant",
					Optional: true,
				},
			},
		},
	}
}

func (e *Engine) NewResourceFromURN(urn *urnx.URN) (types.Resource, error) {
	if urn.Namespace != e.namespace {
		return types.Resource{}, errorInvalidNamespace
	}

	out := types.Resource{
		Type: urn.ResourceType,
		ID:   urn.ResourceID,
	}

	return out, nil
}
