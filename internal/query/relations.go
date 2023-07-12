package query

import (
	"context"
	"io"
	"strings"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.infratographer.com/permissions-api/internal/types"
	"go.infratographer.com/x/gidx"
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

	e.logger.Infow("validation relationship", "sub", subjType.Name, "rel", rel.Relation, "res", resType.Name)

	for _, typeRel := range resType.Relationships {
		// If we find a relation with a name and type that matches our relationship,
		// return
		if rel.Relation == typeRel.Relation {
			for _, typeName := range typeRel.Types {
				if subjType.Name == typeName {
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
			e.subjectRoleRelCreate(subject, role),
		},
	}
	r, err := e.client.WriteRelationships(ctx, request)

	if err != nil {
		return "", err
	}

	return r.WrittenAt.GetToken(), nil
}

// UnassignSubjectRole removes the given role from the given subject.
func (e *engine) UnassignSubjectRole(ctx context.Context, subject types.Resource, role types.Role) (string, error) {
	request := &pb.DeleteRelationshipsRequest{
		RelationshipFilter: e.subjectRoleRelDelete(subject, role),
	}
	r, err := e.client.DeleteRelationships(ctx, request)

	if err != nil {
		return "", err
	}

	return r.DeletedAt.GetToken(), nil
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
		err := e.validateRelationship(rel)
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

func (e *engine) relationshipsToNonRoles(rels []*pb.Relationship, res types.Resource) ([]types.Relationship, error) {
	var out []types.Relationship

	for _, rel := range rels {
		if rel.Subject.Object.ObjectType == e.namespace+"/role" {
			continue
		}

		id, err := gidx.Parse(rel.Subject.Object.ObjectId)
		if err != nil {
			return nil, err
		}

		subj, err := e.NewResourceFromID(id)
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
