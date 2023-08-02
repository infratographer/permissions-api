package mockpermissions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.infratographer.com/permissions-api/pkg/permissions"
	"go.infratographer.com/permissions-api/pkg/permissions/mockpermissions"
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
)

func TestPermissions(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		mockPerms := new(mockpermissions.MockPermissions)

		ctx := mockPerms.ContextWithHandler(context.Background())

		relation := events.AuthRelationshipRelation{
			Relation:  "parent",
			SubjectID: "tnntten-abc",
		}

		mockPerms.On("CreateAuthRelationships", "test", gidx.PrefixedID("tnntten-abc123"), relation).Return(nil)

		err := permissions.CreateAuthRelationships(ctx, "test", "tnntten-abc123", relation)
		require.NoError(t, err)

		mockPerms.AssertExpectations(t)
	})
	t.Run("delete", func(t *testing.T) {
		mockPerms := new(mockpermissions.MockPermissions)

		ctx := mockPerms.ContextWithHandler(context.Background())

		relation := events.AuthRelationshipRelation{
			Relation:  "parent",
			SubjectID: "tnntten-abc",
		}

		mockPerms.On("DeleteAuthRelationships", "test", gidx.PrefixedID("tnntten-abc123"), relation).Return(nil)

		err := permissions.DeleteAuthRelationships(ctx, "test", "tnntten-abc123", relation)
		require.NoError(t, err)

		mockPerms.AssertExpectations(t)
	})
}
