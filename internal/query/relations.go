package query

import (
	"context"
	"fmt"
	"io"
	"strings"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.infratographer.com/permissions-api/internal/types"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
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

	e.logger.Debugw("validation relationship", "sub", subjType.Name, "rel", rel.Relation, "res", resType.Name)

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
func (e *engine) SubjectHasPermission(ctx context.Context, subject types.Resource, action string, resource types.Resource) error {
	req := &pb.CheckPermissionRequest{
		Consistency: &pb.Consistency{
			Requirement: &pb.Consistency_FullyConsistent{
				FullyConsistent: true,
			},
		},
		Resource:   resourceToSpiceDBRef(e.namespace, resource),
		Permission: action,
		Subject: &pb.SubjectReference{
			Object: resourceToSpiceDBRef(e.namespace, subject),
		},
	}

	return e.checkPermission(ctx, req)
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

func (e *engine) checkPermission(ctx context.Context, req *pb.CheckPermissionRequest) error {
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
	ctx, span := tracer.Start(ctx, "engine.CreateRelationships", trace.WithAttributes(attribute.Int("relationships", len(rels))))

	defer span.End()

	for _, rel := range rels {
		err := e.validateRelationship(rel)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return "", err
		}
	}

	relUpdates := e.relationshipsToUpdates(rels)

	request := &pb.WriteRelationshipsRequest{
		Updates: relUpdates,
	}

	r, err := e.client.WriteRelationships(ctx, request)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

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

// DeleteRelationships removes the specified relationships.
// If any relationships fails to be deleted, all completed deletions are re-created.
func (e *engine) DeleteRelationships(ctx context.Context, relationships ...types.Relationship) (string, error) {
	ctx, span := tracer.Start(ctx, "engine.DeleteRelationships", trace.WithAttributes(attribute.Int("relationships", len(relationships))))

	defer span.End()

	var errors []error

	span.AddEvent("validating relationships")

	for i, relationship := range relationships {
		err := e.validateRelationship(relationship)
		if err != nil {
			err = fmt.Errorf("%w: invalid relationship %d", err, i)

			span.RecordError(err)

			errors = append(errors, err)
		}
	}

	if len(errors) != 0 {
		span.SetStatus(codes.Error, "invalid relationships")

		return "", multierr.Combine(errors...)
	}

	errors = []error{}

	var (
		complete   []types.Relationship
		queryToken string
		dErr       error
		cErr       error
	)

	span.AddEvent("deleting relationships")

	for i, relationship := range relationships {
		resType := e.namespace + "/" + relationship.Resource.Type
		subjType := e.namespace + "/" + relationship.Subject.Type

		filter := &pb.RelationshipFilter{
			ResourceType:       resType,
			OptionalResourceId: relationship.Resource.ID.String(),
			OptionalRelation:   relationship.Relation,
			OptionalSubjectFilter: &pb.SubjectFilter{
				SubjectType:       subjType,
				OptionalSubjectId: relationship.Subject.ID.String(),
			},
		}

		queryToken, dErr = e.deleteRelationships(ctx, filter)
		if dErr != nil {
			e.logger.Errorf("%w: failed to delete relationship %d reverting %d completed deletes", dErr, i, len(complete))

			err := fmt.Errorf("%w: failed to delete relationship %d", dErr, i)

			span.RecordError(err)

			errors = append(errors, err)

			break
		}

		complete = append(complete, relationship)
	}

	if len(errors) != 0 {
		span.SetStatus(codes.Error, "error occurred deleting relationships")

		if len(complete) != 0 {
			span.AddEvent("recreating deleted relationships")

			_, cErr = e.CreateRelationships(ctx, complete)
			if cErr != nil {
				e.logger.Error("%w: failed to revert %d deleted relationships", cErr, len(complete))

				err := fmt.Errorf("%w: failed to revert deleted relationships", cErr)

				span.RecordError(err)

				errors = append(errors, err)
			}
		}

		return "", multierr.Combine(errors...)
	}

	return queryToken, nil
}

// DeleteResourceRelationships deletes all relationships originating from the given resource.
func (e *engine) DeleteResourceRelationships(ctx context.Context, resource types.Resource) (string, error) {
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

func (e *engine) relationshipsToNonRoles(rels []*pb.Relationship) ([]types.Relationship, error) {
	var out []types.Relationship

	for _, rel := range rels {
		if rel.Subject.Object.ObjectType == e.namespace+"/role" {
			continue
		}

		resID, err := gidx.Parse(rel.Resource.ObjectId)
		if err != nil {
			return nil, err
		}

		res, err := e.NewResourceFromID(resID)
		if err != nil {
			return nil, err
		}

		subjID, err := gidx.Parse(rel.Subject.Object.ObjectId)
		if err != nil {
			return nil, err
		}

		subj, err := e.NewResourceFromID(subjID)
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

// ListRelationshipsFrom returns all non-role relationships bound to a given resource.
func (e *engine) ListRelationshipsFrom(ctx context.Context, resource types.Resource, queryToken string) ([]types.Relationship, error) {
	resType := e.namespace + "/" + resource.Type

	filter := &pb.RelationshipFilter{
		ResourceType:       resType,
		OptionalResourceId: resource.ID.String(),
	}

	relationships, err := e.readRelationships(ctx, filter, queryToken)
	if err != nil {
		return nil, err
	}

	return e.relationshipsToNonRoles(relationships)
}

// ListRelationshipsTo returns all non-role relationships destined for a given resource.
func (e *engine) ListRelationshipsTo(ctx context.Context, resource types.Resource, queryToken string) ([]types.Relationship, error) {
	relTypes, ok := e.schemaSubjectRelationMap[resource.Type]
	if !ok {
		return nil, ErrInvalidType
	}

	var relationships []*pb.Relationship

	for _, types := range relTypes {
		for _, relType := range types {
			rels, err := e.readRelationships(ctx, &pb.RelationshipFilter{
				ResourceType: e.namespace + "/" + relType,
				OptionalSubjectFilter: &pb.SubjectFilter{
					SubjectType:       e.namespace + "/" + resource.Type,
					OptionalSubjectId: resource.ID.String(),
				},
			}, queryToken)
			if err != nil {
				return nil, err
			}

			relationships = append(relationships, rels...)
		}
	}

	return e.relationshipsToNonRoles(relationships)
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

// listRoleResourceActions returns all resources and action relations for the provided resource type to the provided role.
// Note: The actions returned by this function are the spicedb relationship action.
func (e *engine) listRoleResourceActions(ctx context.Context, role types.Resource, resTypeName string, queryToken string) (map[types.Resource][]string, error) {
	resType := e.namespace + "/" + resTypeName
	roleType := e.namespace + "/role"

	filter := &pb.RelationshipFilter{
		ResourceType: resType,
		OptionalSubjectFilter: &pb.SubjectFilter{
			SubjectType:       roleType,
			OptionalSubjectId: role.ID.String(),
			OptionalRelation: &pb.SubjectFilter_RelationFilter{
				Relation: roleSubjectRelation,
			},
		},
	}

	relationships, err := e.readRelationships(ctx, filter, queryToken)
	if err != nil {
		return nil, err
	}

	resourceIDActions := make(map[gidx.PrefixedID][]string)

	for _, rel := range relationships {
		resourceID, err := gidx.Parse(rel.Resource.ObjectId)
		if err != nil {
			return nil, err
		}

		resourceIDActions[resourceID] = append(resourceIDActions[resourceID], rel.Relation)
	}

	resourceActions := make(map[types.Resource][]string, len(resourceIDActions))

	for resID, actions := range resourceIDActions {
		res, err := e.NewResourceFromID(resID)
		if err != nil {
			return nil, err
		}

		resourceActions[res] = actions
	}

	return resourceActions, nil
}

// GetRole gets the role with it's actions.
func (e *engine) GetRole(ctx context.Context, roleResource types.Resource, queryToken string) (types.Role, error) {
	var (
		resActions map[types.Resource][]string
		err        error
	)

	for _, resType := range e.schemaRoleables {
		resActions, err = e.listRoleResourceActions(ctx, roleResource, resType.Name, queryToken)
		if err != nil {
			return types.Role{}, err
		}

		// roles are only ever created for a single resource, so we can break after the first one is found.
		if len(resActions) != 0 {
			break
		}
	}

	if len(resActions) > 1 {
		return types.Role{}, ErrRoleHasTooManyResources
	}

	// returns the first resources actions.
	for _, actions := range resActions {
		for i, action := range actions {
			actions[i] = relationToAction(action)
		}

		return types.Role{
			ID:      roleResource.ID,
			Actions: actions,
		}, nil
	}

	return types.Role{}, ErrRoleNotFound
}

// GetRoleResource gets the role's assigned resource.
func (e *engine) GetRoleResource(ctx context.Context, roleResource types.Resource, queryToken string) (types.Resource, error) {
	var (
		resActions map[types.Resource][]string
		err        error
	)

	for _, resType := range e.schemaRoleables {
		resActions, err = e.listRoleResourceActions(ctx, roleResource, resType.Name, queryToken)
		if err != nil {
			return types.Resource{}, err
		}

		// roles are only ever created for a single resource, so we can break after the first one is found.
		if len(resActions) != 0 {
			break
		}
	}

	if len(resActions) > 1 {
		return types.Resource{}, ErrRoleHasTooManyResources
	}

	// returns the first resources actions.
	for resource := range resActions {
		return resource, nil
	}

	return types.Resource{}, ErrRoleNotFound
}

// DeleteRole removes all role actions from the assigned resource.
func (e *engine) DeleteRole(ctx context.Context, roleResource types.Resource, queryToken string) (string, error) {
	var (
		resActions map[types.Resource][]string
		err        error
	)

	for _, resType := range e.schemaRoleables {
		resActions, err = e.listRoleResourceActions(ctx, roleResource, resType.Name, queryToken)
		if err != nil {
			return "", err
		}

		// roles are only ever created for a single resource, so we can break after the first one is found.
		if len(resActions) != 0 {
			break
		}
	}

	if len(resActions) == 0 {
		return "", ErrRoleNotFound
	}

	roleType := e.namespace + "/role"

	var filters []*pb.RelationshipFilter

	roleSubjectFilter := &pb.SubjectFilter{
		SubjectType:       roleType,
		OptionalSubjectId: roleResource.ID.String(),
		OptionalRelation: &pb.SubjectFilter_RelationFilter{
			Relation: roleSubjectRelation,
		},
	}

	for resource, relActions := range resActions {
		for _, relAction := range relActions {
			filters = append(filters, &pb.RelationshipFilter{
				ResourceType:          e.namespace + "/" + resource.Type,
				OptionalResourceId:    resource.ID.String(),
				OptionalRelation:      relAction,
				OptionalSubjectFilter: roleSubjectFilter,
			})
		}
	}

	for _, filter := range filters {
		queryToken, err = e.deleteRelationships(ctx, filter)
		if err != nil {
			return "", fmt.Errorf("failed to delete role action %s: %w", filter.OptionalResourceId, err)
		}
	}

	return queryToken, nil
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
