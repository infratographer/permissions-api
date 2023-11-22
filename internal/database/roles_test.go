package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.infratographer.com/x/gidx"

	"go.infratographer.com/permissions-api/internal/database"
	"go.infratographer.com/permissions-api/internal/database/testdb"
)

func TestGetRoleByID(t *testing.T) {
	db, dbClose := testdb.NewTestDatabase(t)
	defer dbClose()

	ctx := context.Background()
	actorID := gidx.PrefixedID("idntusr-abc123")
	roleID := gidx.PrefixedID("permrol-def456")
	roleName := "admins"
	resourceID := gidx.PrefixedID("testten-jkl789")

	// ensure expected empty results returned
	role, err := db.GetRoleByID(ctx, roleID)

	require.Error(t, err, "error expected when no role is found")
	assert.ErrorIs(t, err, database.ErrNoRoleFound)
	require.Nil(t, role, "no role expected to be returned")

	createdRole, err := db.CreateRole(ctx, actorID, roleID, roleName, resourceID)

	require.NoError(t, err, "no error expected while seeding database role")

	err = createdRole.Commit()

	require.NoError(t, err, "no error expected while committing role creation")

	role, err = db.GetRoleByID(ctx, roleID)

	require.NoError(t, err, "no error expected while retrieving role")

	require.NotNil(t, role, "role expected to be returned")

	assert.Equal(t, roleID, role.ID)
	assert.Equal(t, roleName, role.Name)
	assert.Equal(t, resourceID, role.ResourceID)
	assert.Equal(t, actorID, role.CreatorID)
	assert.Equal(t, createdRole.CreatedAt, role.CreatedAt)
	assert.Equal(t, createdRole.UpdatedAt, role.UpdatedAt)
}

func TestGetResourceRoleByName(t *testing.T) {
	db, dbClose := testdb.NewTestDatabase(t)
	defer dbClose()

	ctx := context.Background()
	actorID := gidx.PrefixedID("idntusr-abc123")
	roleID := gidx.PrefixedID("permrol-def456")
	roleName := "admins"
	resourceID := gidx.PrefixedID("testten-jkl789")

	// ensure expected empty results returned
	role, err := db.GetResourceRoleByName(ctx, resourceID, "admins")

	require.Error(t, err, "error expected when no role is found")
	assert.ErrorIs(t, err, database.ErrNoRoleFound)
	require.Nil(t, role, "role expected to be returned")

	createdRole, err := db.CreateRole(ctx, actorID, roleID, roleName, resourceID)

	require.NoError(t, err, "no error expected while seeding database role")

	err = createdRole.Commit()

	require.NoError(t, err, "no error expected while committing role creation")

	role, err = db.GetResourceRoleByName(ctx, resourceID, "admins")

	require.NoError(t, err, "no error expected while retrieving role")

	require.NotNil(t, role, "role expected to be returned")

	assert.Equal(t, roleID, role.ID)
	assert.Equal(t, roleName, role.Name)
	assert.Equal(t, resourceID, role.ResourceID)
	assert.Equal(t, actorID, role.CreatorID)
	assert.Equal(t, createdRole.CreatedAt, role.CreatedAt)
	assert.Equal(t, createdRole.UpdatedAt, role.UpdatedAt)
}

func TestListResourceRoles(t *testing.T) {
	db, dbClose := testdb.NewTestDatabase(t)
	defer dbClose()

	ctx := context.Background()
	actorID := gidx.PrefixedID("idntusr-abc123")

	resourceID := gidx.PrefixedID("testten-jkl789")

	// ensure expected empty results returned
	roles, err := db.ListResourceRoles(ctx, resourceID)

	require.NoError(t, err, "no error expected while retrieving resource roles")
	require.Len(t, roles, 0, "no roles should be returned before they're created")

	groups := map[string]gidx.PrefixedID{
		"super-admins": "permrol-abc123",
		"admins":       "permrol-def456",
		"users":        "permrol-ghi789",
	}

	for roleName, roleID := range groups {
		role, err := db.CreateRole(ctx, actorID, roleID, roleName, resourceID)

		require.NoError(t, err, "no error expected creating role", roleName)

		err = role.Commit()

		require.NoError(t, err, "no error expected while committing role", roleName)
	}

	roles, err = db.ListResourceRoles(ctx, resourceID)

	require.NoError(t, err, "no error expected while retrieving resource roles")

	assert.Len(t, roles, len(groups), "expected returned roles to match group count")

	for _, role := range roles {
		require.NotNil(t, role, "role expected to be returned")

		assert.Equal(t, groups[role.Name], role.ID)
		assert.NotEmpty(t, role.Name)
		assert.Equal(t, resourceID, role.ResourceID)
		assert.Equal(t, actorID, role.CreatorID)
		assert.False(t, role.CreatedAt.IsZero())
		assert.False(t, role.UpdatedAt.IsZero())
	}
}

func TestCreateRole(t *testing.T) {
	db, dbClose := testdb.NewTestDatabase(t)
	defer dbClose()

	ctx := context.Background()
	actorID := gidx.PrefixedID("idntusr-abc123")
	roleID := gidx.PrefixedID("permrol-def456")
	roleID2 := gidx.PrefixedID("permrole-lmn789")
	roleName := "admins"
	resourceID := gidx.PrefixedID("testten-jkl789")

	role, err := db.CreateRole(ctx, actorID, roleID, roleName, resourceID)

	require.NoError(t, err, "no error expected while creating role")

	err = role.Commit()

	require.NoError(t, err, "no error expected while committing role creation")

	assert.Equal(t, roleID, role.ID)
	assert.Equal(t, roleName, role.Name)
	assert.Equal(t, resourceID, role.ResourceID)
	assert.Equal(t, actorID, role.CreatorID)
	assert.False(t, role.CreatedAt.IsZero())
	assert.False(t, role.UpdatedAt.IsZero())

	dupeRole, err := db.CreateRole(ctx, actorID, roleID, roleName, resourceID)

	assert.Error(t, err, "expected error for duplicate index")
	assert.ErrorIs(t, err, database.ErrRoleAlreadyExists, "expected error to be for role already exists")
	require.Nil(t, dupeRole, "expected role to be nil")

	takenNameRole, err := db.CreateRole(ctx, actorID, roleID2, roleName, resourceID)

	assert.Error(t, err, "expected error for already taken name")
	assert.ErrorIs(t, err, database.ErrRoleNameTaken, "expected error to be for already taken name")
	require.Nil(t, takenNameRole, "expected role to be nil")
}

func TestUpdateRole(t *testing.T) {
	db, dbClose := testdb.NewTestDatabase(t)
	defer dbClose()

	ctx := context.Background()

	createActorID := gidx.PrefixedID("idntusr-abc123")
	roleID1 := gidx.PrefixedID("permrol-def456")
	roleID2 := gidx.PrefixedID("permrol-mno753")
	roleName := "admins"
	roleName2 := "temps"
	resourceID := gidx.PrefixedID("testten-jkl789")

	createdRole, err := db.CreateRole(ctx, createActorID, roleID1, roleName, resourceID)
	require.NoError(t, err, "no error expected while seeding database role")

	err = createdRole.Commit()

	require.NoError(t, err, "no error expected while committing role creation")

	createdRole2, err := db.CreateRole(ctx, createActorID, roleID2, roleName2, resourceID)
	require.NoError(t, err, "no error expected while seeding database role 2")

	err = createdRole2.Commit()

	require.NoError(t, err, "no error expected while committing role 2 creation")

	updateActorID := gidx.PrefixedID("idntusr-abc456")

	t.Run("update error", func(t *testing.T) {
		role, err := db.UpdateRole(ctx, updateActorID, roleID2, roleName, resourceID)

		assert.Error(t, err, "expected error updating role name to an already taken role name")
		assert.ErrorIs(t, err, database.ErrRoleNameTaken, "expected error to be role name taken error")
		assert.Nil(t, role, "expected role to be nil")
	})

	updateRoleName := "new-admins"
	updateResourceID := gidx.PrefixedID("testten-mno101")

	t.Run("existing role", func(t *testing.T) {
		role, err := db.UpdateRole(ctx, updateActorID, roleID1, updateRoleName, updateResourceID)

		require.NoError(t, err, "no error expected while updating role")

		require.NotNil(t, role, "role expected to be returned")

		err = role.Commit()

		require.NoError(t, err, "no error expected while committing role update")

		assert.Equal(t, roleID1, role.ID)
		assert.Equal(t, updateRoleName, role.Name)
		assert.Equal(t, updateResourceID, role.ResourceID)
		assert.Equal(t, createActorID, role.CreatorID)
		assert.Equal(t, createdRole.CreatedAt, role.CreatedAt)
		assert.NotEqual(t, createdRole.UpdatedAt, role.UpdatedAt)
	})

	t.Run("new role", func(t *testing.T) {
		newRoleID := gidx.PrefixedID("permrol-xyz789")
		newRoleName := "users"
		newResourceID := gidx.PrefixedID("testten-lmn159")

		role, err := db.UpdateRole(ctx, updateActorID, newRoleID, newRoleName, newResourceID)

		require.NoError(t, err, "no error expected while updating role")

		require.NotNil(t, role, "role expected to be returned")

		err = role.Commit()

		require.NoError(t, err, "no error expected while committing new role from update")

		assert.Equal(t, newRoleID, role.ID)
		assert.Equal(t, newRoleName, role.Name)
		assert.Equal(t, newResourceID, role.ResourceID)
		assert.Equal(t, updateActorID, role.CreatorID)
		assert.False(t, createdRole.CreatedAt.IsZero())
		assert.False(t, createdRole.UpdatedAt.IsZero())
	})
}

func TestDeleteRole(t *testing.T) {
	db, dbClose := testdb.NewTestDatabase(t)
	defer dbClose()

	ctx := context.Background()
	actorID := gidx.PrefixedID("idntusr-abc123")
	roleID := gidx.PrefixedID("permrol-def456")
	roleName := "admins"
	resourceID := gidx.PrefixedID("testten-jkl789")

	_, err := db.DeleteRole(ctx, roleID)

	require.Error(t, err, "error expected while deleting role which doesn't exist")
	require.ErrorIs(t, err, database.ErrNoRoleFound, "expected no role found error for missing role")

	role, err := db.CreateRole(ctx, actorID, roleID, roleName, resourceID)

	require.NoError(t, err, "no error expected while seeding database role")

	err = role.Commit()

	require.NoError(t, err, "no error expected while committing role creation")

	role, err = db.DeleteRole(ctx, roleID)

	require.NoError(t, err, "no error expected while deleting role")

	err = role.Commit()

	require.NoError(t, err, "no error expected while committing role deletion")

	role, err = db.GetRoleByID(ctx, roleID)

	require.Error(t, err, "expected error retrieving role")
	assert.ErrorIs(t, err, database.ErrNoRoleFound, "expected no rows error")
	assert.Nil(t, role, "role expected to nil")
}
