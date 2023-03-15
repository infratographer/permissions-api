package query

import (
	"context"
	"errors"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
)

var ErrScopeNotAssigned = errors.New("the actor does not have permissions to complete this request")

type Stores struct {
	SpiceDB       *authzed.Client
	SpiceDBPrefix string
}

// func (s *Stores) prefixedObjectType(objectType string) string {
// 	if s.SpiceDBPrefix != "" {
// 		return fmt.Sprintf("%s/%s", strings.TrimPrefix(s.SpiceDBPrefix, "/"), objectType)
// 	}

// 	return objectType
// }

func globalPermissions() *pb.ObjectReference {
	return &pb.ObjectReference{
		ObjectType: "global_scope",
		ObjectId:   "infratographer",
	}
}

func AssignGlobalScope(ctx context.Context, db *Stores, actor *Resource, scope string) (string, error) {
	if scope == "create_root_tenant" {
		scope = "root_tenant_creator"
	}

	request := &pb.WriteRelationshipsRequest{Updates: []*pb.RelationshipUpdate{
		{
			Operation: pb.RelationshipUpdate_OPERATION_CREATE,
			Relationship: &pb.Relationship{
				Resource: globalPermissions(),
				Relation: scope,
				Subject: &pb.SubjectReference{
					Object: actor.spiceDBObjectReference(),
				},
			},
		},
	}}

	r, err := db.SpiceDB.WriteRelationships(ctx, request)
	if err != nil {
		return "", err
	}

	return r.WrittenAt.GetToken(), nil
}
