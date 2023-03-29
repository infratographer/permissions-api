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
	errorInvalidNamespace    = errors.New("invalid namespace")
	errorInvalidType         = errors.New("invalid type")
	errorInvalidRelationship = errors.New("invalid relationship")
)

func getTypeForResource(res types.Resource) (types.ResourceType, error) {
	resTypes := GetResourceTypes()

	for _, resType := range resTypes {
		if res.Type == resType.Name {
			return resType, nil
		}
	}

	return types.ResourceType{}, errorInvalidType
}

func validateRelationship(rel types.Relationship) error {
	subjType, err := getTypeForResource(rel.Subject)
	if err != nil {
		return err
	}

	resType, err := getTypeForResource(rel.Resource)
	if err != nil {
		return err
	}

	for _, typeRel := range subjType.Relationships {
		// If we find a relation with a name and type that matches our relationship,
		// return
		if rel.Relation == typeRel.Name && resType.Name == typeRel.Type {
			return nil
		}
	}

	// No matching relationship was found, so we should return an error
	return errorInvalidRelationship
}

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

// AssignSubjectRole assigns the given role to the given subject.
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

// CreateRelationships atomically creates the given relationships in SpiceDB.
func (e *Engine) CreateRelationships(ctx context.Context, rels []types.Relationship) (string, error) {
	for _, rel := range rels {
		err := validateRelationship(rel)
		if err != nil {
			return "", err
		}
	}

	relUpdates := e.relationshipsToUpdates(rels)

	request := &pb.WriteRelationshipsRequest{
		Updates: relUpdates,
	}

	r, err := e.client.WriteRelationships(ctx, request)
	if err != nil {
		return "", err
	}

	return r.WrittenAt.GetToken(), nil
}

// CreateRole creates a role scoped to the given resource with the given actions.
func (e *Engine) CreateRole(ctx context.Context, res types.Resource, actions []string) (types.Role, string, error) {
	role := newRole(actions)
	roleRels := e.roleRelationships(role, res)

	request := &pb.WriteRelationshipsRequest{Updates: roleRels}

	r, err := e.client.WriteRelationships(ctx, request)
	if err != nil {
		return types.Role{}, "", err
	}

	return role, r.WrittenAt.GetToken(), nil
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

func (e *Engine) relationshipsToUpdates(rels []types.Relationship) []*pb.RelationshipUpdate {
	relUpdates := make([]*pb.RelationshipUpdate, len(rels))

	for i, rel := range rels {
		subjRef := resourceToSpiceDBRef(e.namespace, rel.Subject)
		resRef := resourceToSpiceDBRef(e.namespace, rel.Resource)

		relUpdates[i] = &pb.RelationshipUpdate{
			Operation: pb.RelationshipUpdate_OPERATION_CREATE,
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

// GetResourceTypes returns the list of resource types.
func GetResourceTypes() []types.ResourceType {
	return []types.ResourceType{
		{
			Name: "tenant",
			Relationships: []types.ResourceTypeRelationship{
				{
					Name: "tenant",
					Type: "tenant",
				},
			},
		},
		{
			Name: "subject",
		},
	}
}

// NewResourceFromURN returns a new resource struct from a given urn
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
