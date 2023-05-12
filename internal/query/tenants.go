package query

import (
	"context"
	"errors"
	"io"
	"strings"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/google/uuid"
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

func spiceDBRefToResource(namespace string, ref *pb.ObjectReference) (types.Resource, error) {
	sep := namespace + "/"
	before, typeName, found := strings.Cut(ref.ObjectType, sep)

	if !found || before != "" {
		return types.Resource{}, ErrInvalidReference
	}

	resUUID := uuid.MustParse(ref.ObjectId)

	out := types.Resource{
		Type: typeName,
		ID:   resUUID,
	}

	return out, nil
}

// SubjectHasPermission checks if the given subject can do the given action on the given resource
func (e *engine) SubjectHasPermission(ctx context.Context, subject types.Resource, action string, resource types.Resource, queryToken string) error {
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
func (e *engine) AssignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) (string, error) {
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

// ListAssignments returns the assigned subjects for a given role.
func (e *engine) ListAssignments(ctx context.Context, role types.Role, queryToken string) ([]types.Resource, error) {
	roleType := e.namespace + "/role"
	filter := &pb.RelationshipFilter{
		ResourceType:       roleType,
		OptionalResourceId: role.ID.String(),
		OptionalRelation:   roleSubjectRelation,
	}

	relationships, err := e.readRelationships(ctx, filter, queryToken)
	if err != nil {
		return nil, err
	}

	out := make([]types.Resource, len(relationships))

	for i, rel := range relationships {
		subjRes, err := spiceDBRefToResource(e.namespace, rel.Subject.Object)
		if err != nil {
			return nil, err
		}

		out[i] = subjRes
	}

	return out, nil
}

func (e *engine) subjectRoleRel(subject types.Resource, role types.Role) *pb.RelationshipUpdate {
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

func (e *engine) checkPermission(ctx context.Context, req *pb.CheckPermissionRequest, queryToken string) error {
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
func (e *engine) CreateRelationships(ctx context.Context, rels []types.Relationship) (string, error) {
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
func (e *engine) CreateRole(ctx context.Context, res types.Resource, actions []string) (types.Role, string, error) {
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

func relationToAction(relation string) string {
	action, _, found := strings.Cut(relation, "_rel")

	if !found {
		panic("unexpected relation on role")
	}

	return action
}

func (e *engine) roleRelationships(role types.Role, resource types.Resource) []*pb.RelationshipUpdate {
	var rels []*pb.RelationshipUpdate

	roleResource := types.Resource{
		Type: "role",
		ID:   role.ID,
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

func (e *engine) readRelationships(ctx context.Context, filter *pb.RelationshipFilter, queryToken string) ([]*pb.Relationship, error) {
	var req pb.ReadRelationshipsRequest

	if queryToken != "" {
		req.Consistency = &pb.Consistency{
			Requirement: &pb.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &pb.ZedToken{
					Token: queryToken,
				},
			},
		}
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

// DeleteRelationships deletes all relationships originating from the given resource.
func (e *engine) DeleteRelationships(ctx context.Context, resource types.Resource) (string, error) {
	resType := e.namespace + "/" + resource.Type

	filter := &pb.RelationshipFilter{
		ResourceType:       resType,
		OptionalResourceId: resource.ID.String(),
	}

	return e.deleteRelationships(ctx, filter)
}

func (e *engine) deleteRelationships(ctx context.Context, filter *pb.RelationshipFilter) (string, error) {
	request := &pb.DeleteRelationshipsRequest{
		RelationshipFilter: filter,
	}
	r, err := e.client.DeleteRelationships(ctx, request)

	if err != nil {
		return "", err
	}

	return r.DeletedAt.GetToken(), nil
}

func relationshipsToRoles(rels []*pb.Relationship) []types.Role {
	var roleIDs []uuid.UUID

	roleMap := make(map[uuid.UUID]*types.Role)

	for _, rel := range rels {
		roleIDStr := rel.Subject.Object.ObjectId
		roleID := uuid.MustParse(roleIDStr)
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

func (e *engine) relationshipsToNonRoles(rels []*pb.Relationship, res types.Resource) ([]types.Relationship, error) {
	var out []types.Relationship

	for _, rel := range rels {
		if rel.Subject.Object.ObjectType == e.namespace+"/role" {
			continue
		}

		subjRes, err := spiceDBRefToResource(e.namespace, rel.Subject.Object)
		if err != nil {
			return nil, err
		}

		item := types.Relationship{
			Resource: res,
			Relation: rel.Relation,
			Subject:  subjRes,
		}

		out = append(out, item)
	}

	return out, nil
}

// ListRelationships returns all non-role relationships bound to a given resource.
func (e *engine) ListRelationships(ctx context.Context, resource types.Resource, queryToken string) ([]types.Relationship, error) {
	resType := e.namespace + "/" + resource.Type

	filter := &pb.RelationshipFilter{
		ResourceType:       resType,
		OptionalResourceId: resource.ID.String(),
	}

	relationships, err := e.readRelationships(ctx, filter, queryToken)
	if err != nil {
		return nil, err
	}

	return e.relationshipsToNonRoles(relationships, resource)
}

// ListRoles returns all roles bound to a given resource.
func (e *engine) ListRoles(ctx context.Context, resource types.Resource, queryToken string) ([]types.Role, error) {
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

	relationships, err := e.readRelationships(ctx, filter, queryToken)
	if err != nil {
		return nil, err
	}

	out := relationshipsToRoles(relationships)

	return out, nil
}

// GetResourceTypes returns the list of resource types.
func GetResourceTypes() []types.ResourceType {
	return []types.ResourceType{
		{
			Name: "loadbalancer",
			Relationships: []types.ResourceTypeRelationship{
				{
					Name: "tenant",
					Type: "tenant",
				},
			},
		},
		{
			Name: "role",
			Relationships: []types.ResourceTypeRelationship{
				{
					Name: "tenant",
					Type: "tenant",
				},
			},
		},
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
			Name: "user",
		},
		{
			Name: "client",
		},
	}
}

// NewResourceFromURN returns a new resource struct from a given urn
func (e *engine) NewResourceFromURN(urn *urnx.URN) (types.Resource, error) {
	if urn.Namespace != e.namespace {
		return types.Resource{}, errorInvalidNamespace
	}

	out := types.Resource{
		Type: urn.ResourceType,
		ID:   urn.ResourceID,
	}

	return out, nil
}

// NewURNFromResource creates a new URN namespaced to the given engine from the given resource.
func (e *engine) NewURNFromResource(res types.Resource) (*urnx.URN, error) {
	return urnx.Build(e.namespace, res.Type, res.ID)
}
