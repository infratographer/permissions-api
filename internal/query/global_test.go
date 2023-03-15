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
	t.Helper()

	grpcPass := "infradev"

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
	t.Helper()

	for _, dbType := range []string{"global_scope", "user", "service_account", "role", "tenant", "instance", "ip_block", "ip_address"} {
		delRequest := &pb.DeleteRelationshipsRequest{RelationshipFilter: &pb.RelationshipFilter{ResourceType: dbType}}
		_, err := client.DeleteRelationships(ctx, delRequest)
		require.NoError(t, err, "failure deleting relationships")
	}
}

func TestActorGlobalScopes(t *testing.T) {
	ctx := context.Background()
	s := dbTest(ctx, t)

	var err error

	userRes, err := query.NewResourceFromURN("urn:infratographer:user:" + uuid.NewString())
	require.NoError(t, err)

	queryToken := ""

	t.Run("add a global scope to a user ", func(t *testing.T) {
		queryToken, err = query.AssignGlobalScope(ctx, s, userRes, "create_root_tenant")
		assert.NoError(t, err)
	})

	t.Run("check that the user has the global scope", func(t *testing.T) {
		err := query.ActorHasGlobalPermission(ctx, s.SpiceDB, userRes, "create_root_tenant", queryToken)
		assert.NoError(t, err)
	})

	t.Run("error returned when the user doesn't have the global scope", func(t *testing.T) {
		newUser, err := query.NewResourceFromURN("urn:infratographer:user:" + uuid.NewString())
		require.NoError(t, err)

		err = query.ActorHasGlobalPermission(ctx, s.SpiceDB, newUser, "create_root_tenant", queryToken)
		assert.Error(t, err)
		assert.ErrorIs(t, err, query.ErrScopeNotAssigned)
	})
}
