package query

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
)

var roleActorRelation = "actor"

var (
	BuiltInRoleAdmins  = "Admins"
	BuiltInRoleEditors = "Editors"
	BuiltInRoleViewers = "Viewers"
)

func ActorResourceList(ctx context.Context, db *authzed.Client, actorURN, resourceURNPrefix, scope, queryToken string) ([]string, error) {
	rt, err := ResourceTypeByURN(resourceURNPrefix)
	if err != nil {
		return []string{}, err
	}

	actor, err := NewResourceFromURN(actorURN)
	if err != nil {
		return []string{}, err
	}

	req := &pb.LookupResourcesRequest{
		ResourceObjectType: rt.DBType,
		Permission:         scope,
		Subject: &pb.SubjectReference{
			Object: actor.spiceDBObjectReference(),
			// OptionalRelation: subjectRel,
		},
	}

	if queryToken != "" {
		req.Consistency = &pb.Consistency{Requirement: &pb.Consistency_AtLeastAsFresh{AtLeastAsFresh: &pb.ZedToken{Token: queryToken}}}
	}

	respStream, err := db.LookupResources(ctx, req)
	if err != nil {
		return []string{}, err
	}

	resources := []string{}

	for {
		resp, err := respStream.Recv()

		switch {
		case errors.Is(err, io.EOF):
			return resources, nil
		case err != nil:
			return []string{}, err
		default:
			resources = append(resources, resp.ResourceObjectId)
		}
	}
}

func ActorHasGlobalPermission(ctx context.Context, db *authzed.Client, actor *Resource, scope, queryToken string) error {
	req := &pb.CheckPermissionRequest{
		Resource:   globalPermissions(),
		Permission: scope,
		Subject: &pb.SubjectReference{
			Object: actor.spiceDBObjectReference(),
		},
	}

	return checkPermission(ctx, db, req, queryToken)
}

func ActorHasPermission(ctx context.Context, db *authzed.Client, actor *Resource, scope string, object *Resource, queryToken string) error {
	req := &pb.CheckPermissionRequest{
		Resource:   object.spiceDBObjectReference(),
		Permission: scope,
		Subject: &pb.SubjectReference{
			Object: actor.spiceDBObjectReference(),
		},
	}

	return checkPermission(ctx, db, req, queryToken)
}

func AssignActorRole(ctx context.Context, db *authzed.Client, actor *Resource, role string, object *Resource) (string, error) {
	request := &pb.WriteRelationshipsRequest{Updates: []*pb.RelationshipUpdate{actorRoleRel(actor, role, object)}}
	r, err := db.WriteRelationships(ctx, request)
	if err != nil {
		return "", err
	}

	return r.WrittenAt.GetToken(), nil
}

func actorRoleRel(actor *Resource, role string, object *Resource) *pb.RelationshipUpdate {
	return &pb.RelationshipUpdate{
		Operation: pb.RelationshipUpdate_OPERATION_CREATE,
		Relationship: &pb.Relationship{
			Resource: &pb.ObjectReference{
				ObjectType: "role",
				ObjectId:   dbRoleName(role, object),
			},
			Relation: roleActorRelation,
			Subject: &pb.SubjectReference{
				Object: actor.spiceDBObjectReference(),
			},
		},
	}
}

func checkPermission(ctx context.Context, db *authzed.Client, req *pb.CheckPermissionRequest, queryToken string) error {
	if queryToken != "" {
		req.Consistency = &pb.Consistency{Requirement: &pb.Consistency_AtLeastAsFresh{AtLeastAsFresh: &pb.ZedToken{Token: queryToken}}}
	}

	resp, err := db.CheckPermission(ctx, req)
	if err != nil {
		return err
	}

	if resp.Permissionship == pb.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION {
		return nil
	}

	return ErrScopeNotAssigned
}

func dbRoleName(role string, res *Resource) string {
	return fmt.Sprintf("%s_%s_%s", res.ResourceType.DBType, res.ID, role)
}

func CreateBuiltInRoles(ctx context.Context, db *authzed.Client, res *Resource) (string, error) {
	rels := builtInRoles(res)

	request := &pb.WriteRelationshipsRequest{Updates: rels}

	r, err := db.WriteRelationships(ctx, request)
	if err != nil {
		return "", err
	}

	return r.WrittenAt.GetToken(), nil
}

func builtInRoles(res *Resource) []*pb.RelationshipUpdate {
	adminRole := &pb.ObjectReference{
		ObjectType: "role",
		ObjectId:   dbRoleName(BuiltInRoleAdmins, res),
	}

	editorRole := &pb.ObjectReference{
		ObjectType: "role",
		ObjectId:   dbRoleName(BuiltInRoleEditors, res),
	}

	viewerRole := &pb.ObjectReference{
		ObjectType: "role",
		ObjectId:   dbRoleName(BuiltInRoleViewers, res),
	}

	adminAssignments := []string{"deleter", "role_editor", "permission_granter", "tenant_creator", "tenant_deleter"}
	editorAssignments := []string{"editor", "instance_creator", "instance_deleter", "instance_editor", "tenant_editor"}
	viewerAssignments := []string{"viewer", "instance_viewer", "tenant_viewer"}

	editorAssignments = append(editorAssignments, viewerAssignments...)
	adminAssignments = append(adminAssignments, editorAssignments...)

	rels := []*pb.RelationshipUpdate{}

	for _, scope := range adminAssignments {
		rels = append(rels, &pb.RelationshipUpdate{
			Operation: pb.RelationshipUpdate_OPERATION_CREATE,
			Relationship: &pb.Relationship{
				Resource: res.spiceDBObjectReference(),
				Relation: scope,
				Subject: &pb.SubjectReference{
					Object:           adminRole,
					OptionalRelation: roleActorRelation,
				},
			},
		})
	}

	for _, scope := range editorAssignments {
		rels = append(rels, &pb.RelationshipUpdate{
			Operation: pb.RelationshipUpdate_OPERATION_CREATE,
			Relationship: &pb.Relationship{
				Resource: res.spiceDBObjectReference(),
				Relation: scope,
				Subject: &pb.SubjectReference{
					Object:           editorRole,
					OptionalRelation: roleActorRelation,
				},
			},
		})
	}

	for _, scope := range viewerAssignments {
		rels = append(rels, &pb.RelationshipUpdate{
			Operation: pb.RelationshipUpdate_OPERATION_CREATE,
			Relationship: &pb.Relationship{
				Resource: res.spiceDBObjectReference(),
				Relation: scope,
				Subject: &pb.SubjectReference{
					Object:           viewerRole,
					OptionalRelation: roleActorRelation,
				},
			},
		})
	}

	for _, role := range []*pb.ObjectReference{adminRole, editorRole, viewerRole} {
		for _, relation := range []string{"built_in_role", "tenant"} {
			rels = append(rels, &pb.RelationshipUpdate{
				Operation: pb.RelationshipUpdate_OPERATION_CREATE,
				Relationship: &pb.Relationship{
					Resource: role,
					Relation: relation,
					Subject: &pb.SubjectReference{
						Object: res.spiceDBObjectReference(),
					},
				},
			})
		}
	}

	return rels
}

type Resource struct {
	URN          string
	ID           string
	ResourceType *ResourceType
	Fields       map[string]string
}

type ResourceType struct {
	Name          string `json:"name"`
	URNPrefix     string `json:"upn_prefix"`
	APIURI        string `json:"api_uri"`
	DBType        string `json:"db_type"`
	Relationships []*ResourceRelationship
}

type ResourceRelationship struct {
	Name       string
	Field      string
	Optional   bool
	DBTypes    string
	DBRelation string
}

func GetResourceTypes() []*ResourceType {
	return []*ResourceType{
		{
			Name:      "Tenant",
			DBType:    "tenant",
			URNPrefix: "urn:infratographer:tenant",
			Relationships: []*ResourceRelationship{
				{
					Name:       "Parent Tenant",
					Field:      "parent_tenant_id",
					DBTypes:    "tenant",
					DBRelation: "parent_tenant",
					Optional:   true,
				},
			},
		},
		{
			Name:      "User",
			DBType:    "user",
			URNPrefix: "urn:infratographer:user",
		},
		{
			Name:      "Token",
			DBType:    "token",
			URNPrefix: "urn:infratographer:token",
		},
		{
			Name:      "Instance",
			DBType:    "instance",
			URNPrefix: "urn:infratographer:instance",
			Relationships: []*ResourceRelationship{
				{
					Name:       "Tenant",
					Field:      "tenant_id",
					DBTypes:    "tenant",
					DBRelation: "tenant",
					Optional:   false,
				},
			},
		},
	}
}

func NewResourceFromURN(urn string) (*Resource, error) {
	parts := strings.Split(urn, ":")

	r := &Resource{
		URN: urn,
		ID:  parts[len(parts)-1],
	}

	prefixParts := parts[:len(parts)-1]

	prefix := strings.Join(prefixParts, ":")

	rt, err := ResourceTypeByURN(prefix)
	if err != nil {
		return nil, err
	}

	r.ResourceType = rt

	return r, nil
}

func ResourceTypeByURN(urn string) (*ResourceType, error) {
	for _, resType := range GetResourceTypes() {
		if resType.URNPrefix == urn {
			return resType, nil
		}
	}

	return nil, errors.New("invalid urn")
}

func (r *Resource) spiceDBObjectReference() *pb.ObjectReference {
	return &pb.ObjectReference{
		ObjectType: r.ResourceType.DBType,
		ObjectId:   r.ID,
	}
}

func CreateSpiceDBRelationships(ctx context.Context, db *authzed.Client, r, actor *Resource) (string, error) {
	rels := []*pb.RelationshipUpdate{}

	if r.ResourceType.URNPrefix == "urn:infratographer:tenant" {
		rels = append(rels, builtInRoles(r)...)

		rels = append(rels, actorRoleRel(actor, BuiltInRoleAdmins, r))
	}

	for _, rr := range r.ResourceType.Relationships {
		if rr.Optional && r.Fields[rr.Field] == "" {
			continue
		}

		if r.Fields[rr.Field] == "" {
			return "", errors.New("missing required relationship to " + rr.Name)
		}

		rels = append(rels, &pb.RelationshipUpdate{
			Operation: pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: &pb.Relationship{
				Resource: r.spiceDBObjectReference(),
				Relation: rr.DBRelation,
				Subject: &pb.SubjectReference{
					Object: &pb.ObjectReference{
						ObjectType: rr.DBTypes,
						ObjectId:   r.Fields[rr.Field],
					},
				},
			},
		})
	}

	res, err := db.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{Updates: rels})
	if err != nil {
		return "", err
	}

	return res.WrittenAt.GetToken(), nil
}
