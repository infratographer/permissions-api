package query_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.infratographer.com/permissions-api/internal/query"
)

func TestActorScopes(t *testing.T) {
	ctx := context.Background()
	s := dbTest(ctx, t)

	var err error

	tenURN := "urn:infratographer:tenant:" + uuid.NewString()
	tenRes, err := query.NewResourceFromURN(tenURN)
	require.NoError(t, err)
	userURN := "urn:infratographer:user:" + uuid.NewString()
	userRes, err := query.NewResourceFromURN(userURN)
	require.NoError(t, err)

	queryToken, err := query.CreateBuiltInRoles(ctx, s.SpiceDB, tenRes)
	assert.NoError(t, err)

	t.Run("allow a user to view an ou", func(t *testing.T) {
		queryToken, err = query.AssignActorRole(ctx, s.SpiceDB, userRes, "Editors", tenRes)
		assert.NoError(t, err)
	})

	t.Run("check that the user has edit access to an ou", func(t *testing.T) {
		err := query.ActorHasPermission(ctx, s.SpiceDB, userRes, "edit", tenRes, queryToken)
		assert.NoError(t, err)
	})

	t.Run("error returned when the user doesn't have the global scope", func(t *testing.T) {
		otherUserRes, err := query.NewResourceFromURN("urn:infratographer:user:" + uuid.NewString())
		require.NoError(t, err)

		err = query.ActorHasPermission(ctx, s.SpiceDB, otherUserRes, "edit", tenRes, queryToken)
		assert.Error(t, err)
		assert.ErrorIs(t, err, query.ErrScopeNotAssigned)
	})

	t.Run("List all the resources a user has access to", func(t *testing.T) {
		list, err := query.ActorResourceList(ctx, s.SpiceDB, userURN, "urn:infratographer:tenant", "edit", queryToken)
		assert.NoError(t, err)

		assert.Len(t, list, 1)
	})
}
