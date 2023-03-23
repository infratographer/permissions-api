package query_test

import (
	"context"
	"testing"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/spicedbx"
)

func dbTest(ctx context.Context, t *testing.T) *query.Stores {
	grpcPass := "infradev"

	// client, err := authzed.NewClient(
	// 	"grpc.authzed.com:443",
	// 	grpcutil.WithSystemCerts(grpcutil.VerifyCA),
	// 	grpcutil.WithBearerToken(grpcPass),
	// )

	client, err := authzed.NewClient(
		"spicedb:50051",
		// NOTE: For SpiceDB behind TLS, use:
		// grpcutil.WithBearerToken("infra"),
		// grpcutil.WithSystemCerts(grpcutil.VerifyCA),
		grpcutil.WithInsecureBearerToken(grpcPass),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
	)
	require.NoError(t, err)

	request := &pb.WriteSchemaRequest{Schema: spicedbx.GeneratedSchema("")}
	_, err = client.WriteSchema(ctx, request)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanDB(ctx, t, client)
	})

	return &query.Stores{SpiceDB: client}
}

func cleanDB(ctx context.Context, t *testing.T, client *authzed.Client) {
	for _, dbType := range []string{"subject", "role", "tenant", "loadbalancer"} {
		delRequest := &pb.DeleteRelationshipsRequest{RelationshipFilter: &pb.RelationshipFilter{ResourceType: dbType}}
		_, err := client.DeleteRelationships(ctx, delRequest)
		require.NoError(t, err, "failure deleting relationships")
	}
}

func TestActorScopes(t *testing.T) {
	ctx := context.Background()
	s := dbTest(ctx, t)

	var err error

	tenURN := "urn:infratographer:tenant:" + uuid.NewString()
	tenRes, err := query.NewResourceFromURN(tenURN)
	require.NoError(t, err)
	userURN := "urn:infratographer:subject:" + uuid.NewString()
	userRes, err := query.NewResourceFromURN(userURN)
	require.NoError(t, err)

	queryToken, err := query.CreateBuiltInRoles(ctx, s.SpiceDB, tenRes)
	assert.NoError(t, err)

	t.Run("allow a user to view an ou", func(t *testing.T) {
		queryToken, err = query.AssignActorRole(ctx, s.SpiceDB, userRes, "Editors", tenRes)
		assert.NoError(t, err)
	})

	t.Run("check that the user has edit access to an ou", func(t *testing.T) {
		err := query.ActorHasPermission(ctx, s.SpiceDB, userRes, "loadbalancer_get", tenRes, queryToken)
		assert.NoError(t, err)
	})

	t.Run("error returned when the user doesn't have the global scope", func(t *testing.T) {
		otherUserRes, err := query.NewResourceFromURN("urn:infratographer:subject:" + uuid.NewString())
		require.NoError(t, err)

		err = query.ActorHasPermission(ctx, s.SpiceDB, otherUserRes, "loadbalancer_get", tenRes, queryToken)
		assert.Error(t, err)
		assert.ErrorIs(t, err, query.ErrScopeNotAssigned)
	})
}
