package query

import (
	"context"
	"testing"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.infratographer.com/permissions-api/internal/spicedbx"
	"go.infratographer.com/x/urnx"
)

func testEngine(ctx context.Context, t *testing.T, namespace string) *Engine {
	config := spicedbx.Config{
		Endpoint: "spicedb:50051",
		Key:      "infradev",
		Insecure: true,
	}

	client, err := spicedbx.NewClient(config, false)
	require.NoError(t, err)

	request := &pb.WriteSchemaRequest{Schema: spicedbx.GeneratedSchema(namespace)}
	_, err = client.WriteSchema(ctx, request)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanDB(ctx, t, client, namespace)
	})

	out := NewEngine(namespace, client)

	return out
}

func cleanDB(ctx context.Context, t *testing.T, client *authzed.Client, namespace string) {
	for _, dbType := range []string{"subject", "role", "tenant"} {
		namespacedType := namespace + "/" + dbType
		delRequest := &pb.DeleteRelationshipsRequest{
			RelationshipFilter: &pb.RelationshipFilter{
				ResourceType: namespacedType,
			},
		}
		_, err := client.DeleteRelationships(ctx, delRequest)
		require.NoError(t, err, "failure deleting relationships")
	}
}

func TestSubjectActions(t *testing.T) {
	ctx := context.Background()
	e := testEngine(ctx, t, "infratographer")

	tenURN, err := urnx.Build("infratographer", "tenant", uuid.New())
	require.NoError(t, err)
	tenRes, err := e.NewResourceFromURN(tenURN)
	require.NoError(t, err)
	subjURN, err := urnx.Build("infratographer", "subject", uuid.New())
	require.NoError(t, err)
	userRes, err := e.NewResourceFromURN(subjURN)
	require.NoError(t, err)

	roles, queryToken, err := e.CreateBuiltInRoles(ctx, tenRes)
	assert.NoError(t, err)

	t.Run("allow a user to view an ou", func(t *testing.T) {
		role := roles[0]
		require.Contains(t, role.Actions, "loadbalancer_update")

		queryToken, err = e.AssignSubjectRole(ctx, userRes, role)
		assert.NoError(t, err)
	})

	t.Run("check that the user has edit access to an ou", func(t *testing.T) {
		err := e.SubjectHasPermission(ctx, userRes, "loadbalancer_update", tenRes, queryToken)
		assert.NoError(t, err)
	})

	t.Run("error returned when the user doesn't have the global action", func(t *testing.T) {
		subjURN, err := urnx.Build("infratographer", "subject", uuid.New())
		require.NoError(t, err)
		otherUserRes, err := e.NewResourceFromURN(subjURN)
		require.NoError(t, err)

		err = e.SubjectHasPermission(ctx, otherUserRes, "loadbalancer_get", tenRes, queryToken)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrActionNotAssigned)
	})
}
