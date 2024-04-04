package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.infratographer.com/x/gidx"

	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/storage/teststore"
	"go.infratographer.com/permissions-api/internal/testingx"
)

func TestGetRoleByID(t *testing.T) {
	store, closeStore := teststore.NewTestStorage(t)

	t.Cleanup(closeStore)

	ctx := context.Background()

	actorID := gidx.PrefixedID("idntusr-abc123")
	resourceID := gidx.PrefixedID("testten-jkl789")
	roleID := gidx.MustNewID("permrol")
	roleName := "users"

	dbCtx, err := store.BeginContext(ctx)
	require.NoError(t, err, "no error expected beginning transaction context")

	createdRole, err := store.CreateRole(dbCtx, actorID, roleID, roleName, resourceID)
	require.NoError(t, err, "no error expected while seeding database role")

	err = store.CommitContext(dbCtx)
	require.NoError(t, err, "no error expected while committing role creation")

	testCases := []testingx.TestCase[gidx.PrefixedID, storage.Role]{
		{
			Name:  "NotFound",
			Input: "permrol-notfound123",
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[storage.Role]) {
				require.Error(t, res.Err, "error expected when no role is found")
				assert.ErrorIs(t, res.Err, storage.ErrNoRoleFound)
				require.Empty(t, res.Success.ID, "no role expected to be returned")
			},
		},
		{
			Name:  "Found",
			Input: roleID,
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[storage.Role]) {
				require.NoError(t, err, "no error expected while retrieving role")

				assert.Equal(t, roleID, res.Success.ID)
				assert.Equal(t, roleName, res.Success.Name)
				assert.Equal(t, resourceID, res.Success.ResourceID)
				assert.Equal(t, actorID, res.Success.CreatedBy)
				assert.Equal(t, createdRole.CreatedAt, res.Success.CreatedAt)
				assert.Equal(t, createdRole.UpdatedAt, res.Success.UpdatedAt)
			},
		},
	}

	testFn := func(ctx context.Context, input gidx.PrefixedID) testingx.TestResult[storage.Role] {
		role, err := store.GetRoleByID(ctx, input)

		return testingx.TestResult[storage.Role]{
			Success: role,
			Err:     err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestListResourceRoles(t *testing.T) {
	store, closeStore := teststore.NewTestStorage(t)

	t.Cleanup(closeStore)

	ctx := context.Background()

	actorID := gidx.PrefixedID("idntusr-abc123")
	resourceID := gidx.PrefixedID("testten-jkl789")

	groups := map[string]gidx.PrefixedID{
		"super-admins": "permrol-abc123",
		"admins":       "permrol-def456",
		"users":        "permrol-ghi789",
	}

	dbCtx, err := store.BeginContext(ctx)
	require.NoError(t, err, "no error expected beginning transaction context")

	for roleName, roleID := range groups {
		_, err := store.CreateRole(dbCtx, actorID, roleID, roleName, resourceID)

		require.NoError(t, err, "no error expected creating role", roleName)
	}

	err = store.CommitContext(dbCtx)
	require.NoError(t, err, "no error expected while committing roles")

	testCases := []testingx.TestCase[gidx.PrefixedID, []storage.Role]{
		{
			Name:  "NoRoles",
			Input: "testten-noroles123",
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[[]storage.Role]) {
				require.NoError(t, res.Err, "no error expected while retrieving resource roles")
				require.Len(t, res.Success, 0, "no roles should be returned before they're created")
			},
		},
		{
			Name:  "FoundRoles",
			Input: resourceID,
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[[]storage.Role]) {
				require.NoError(t, res.Err, "no error expected while retrieving role")

				assert.Len(t, res.Success, len(groups), "expected returned roles to match group count")

				for _, role := range res.Success {
					require.NotEmpty(t, role.ID, "role expected to be returned")

					require.NotEmpty(t, role.Name)
					assert.Equal(t, groups[role.Name], role.ID)
					assert.Equal(t, resourceID, role.ResourceID)
					assert.Equal(t, actorID, role.CreatedBy)
					assert.False(t, role.CreatedAt.IsZero())
					assert.False(t, role.UpdatedAt.IsZero())
				}
			},
		},
	}

	testFn := func(ctx context.Context, input gidx.PrefixedID) testingx.TestResult[[]storage.Role] {
		roles, err := store.ListResourceRoles(ctx, input)

		return testingx.TestResult[[]storage.Role]{
			Success: roles,
			Err:     err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestCreateRole(t *testing.T) {
	store, closeStore := teststore.NewTestStorage(t)

	t.Cleanup(closeStore)

	ctx := context.Background()

	actorID := gidx.PrefixedID("idntusr-abc123")
	resourceID := gidx.PrefixedID("testten-jkl789")

	type testInput struct {
		id   gidx.PrefixedID
		name string
	}

	testCases := []testingx.TestCase[testInput, storage.Role]{
		{
			Name: "Success",
			Input: testInput{
				id:   "permrol-abc123",
				name: "admins",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[storage.Role]) {
				require.NoError(t, res.Err, "no error expected creating role")

				assert.Equal(t, "permrol-abc123", res.Success.ID.String())
				assert.Equal(t, "admins", res.Success.Name)
				assert.Equal(t, resourceID, res.Success.ResourceID)
				assert.Equal(t, actorID, res.Success.CreatedBy)
				assert.False(t, res.Success.CreatedAt.IsZero())
				assert.False(t, res.Success.UpdatedAt.IsZero())
			},
			Sync: true,
		},
		{
			Name: "DuplicateIndex",
			Input: testInput{
				id:   "permrol-abc123",
				name: "users",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[storage.Role]) {
				require.Error(t, res.Err, "expected error for duplicate index")
				assert.ErrorIs(t, res.Err, storage.ErrRoleAlreadyExists, "expected error to be for role already exists")
				require.Empty(t, res.Success.ID, "expected role to be empty")
			},
			Sync: true,
		},
		{
			Name: "NameTaken",
			Input: testInput{
				id:   "permrol-def456",
				name: "admins",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[storage.Role]) {
				assert.Error(t, res.Err, "expected error for already taken name")
				assert.ErrorIs(t, res.Err, storage.ErrRoleNameTaken, "expected error to be for already taken name")
				require.Empty(t, res.Success.ID, "expected role to be empty")
			},
			Sync: true,
		},
	}

	testFn := func(ctx context.Context, input testInput) testingx.TestResult[storage.Role] {
		var result testingx.TestResult[storage.Role]

		dbCtx, err := store.BeginContext(ctx)
		if err != nil {
			result.Err = err

			return result
		}

		result.Success, result.Err = store.CreateRole(dbCtx, actorID, input.id, input.name, resourceID)
		if result.Err != nil {
			store.RollbackContext(dbCtx) //nolint:errcheck // skip check in test

			return result
		}

		result.Err = store.CommitContext(dbCtx)

		return result
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestUpdateRole(t *testing.T) {
	store, closeStore := teststore.NewTestStorage(t)

	t.Cleanup(closeStore)

	ctx := context.Background()

	actorID := gidx.PrefixedID("idntusr-abc123")
	resourceID := gidx.PrefixedID("testten-jkl789")

	role1ID := gidx.PrefixedID("permrol-abc123")
	role1Name := "admins"

	role2ID := gidx.PrefixedID("permrol-def456")
	role2Name := "users"

	dbCtx, err := store.BeginContext(ctx)

	require.NoError(t, err, "no error expected beginning transaction context")

	createdDBRole1, err := store.CreateRole(dbCtx, actorID, role1ID, role1Name, resourceID)
	require.NoError(t, err, "no error expected while seeding database role")

	_, err = store.CreateRole(dbCtx, actorID, role2ID, role2Name, resourceID)
	require.NoError(t, err, "no error expected while seeding database role 2")

	err = store.CommitContext(dbCtx)
	require.NoError(t, err, "no error expected while committing role creations")

	type testInput struct {
		id   gidx.PrefixedID
		name string
	}

	testCases := []testingx.TestCase[testInput, storage.Role]{
		{
			Name: "NameTaken",
			Input: testInput{
				id:   role1ID,
				name: role2Name,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[storage.Role]) {
				assert.Error(t, res.Err, "expected error updating role name to an already taken role name")
				assert.ErrorIs(t, res.Err, storage.ErrRoleNameTaken, "expected error to be role name taken error")
				assert.Empty(t, res.Success.ID, "expected role to be empty")
			},
			Sync: true,
		},
		{
			Name: "Success",
			Input: testInput{
				id:   role1ID,
				name: "root-admins",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[storage.Role]) {
				require.NoError(t, res.Err, "no error expected while updating role")

				assert.Equal(t, role1ID, res.Success.ID)
				assert.Equal(t, "root-admins", res.Success.Name)
				assert.Equal(t, actorID, res.Success.CreatedBy)
				assert.Equal(t, createdDBRole1.CreatedAt, res.Success.CreatedAt)
				assert.NotEqual(t, createdDBRole1.UpdatedAt, res.Success.UpdatedAt)
			},
			Sync: true,
		},
		{
			Name: "NotFound",
			Input: testInput{
				id:   "permrol-notfound789",
				name: "not-found",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[storage.Role]) {
				require.Error(t, res.Err, "an error expected to be returned for an unknown role")
				assert.ErrorIs(t, res.Err, storage.ErrNoRoleFound, "unexpected error returned")
				assert.Empty(t, res.Success.ID, "expected role to be empty")
			},
		},
	}

	testFn := func(ctx context.Context, input testInput) testingx.TestResult[storage.Role] {
		var result testingx.TestResult[storage.Role]

		dbCtx, err := store.BeginContext(ctx)
		if err != nil {
			result.Err = err

			return result
		}

		result.Success, result.Err = store.UpdateRole(dbCtx, actorID, input.id, input.name)
		if result.Err != nil {
			store.RollbackContext(dbCtx) //nolint:errcheck // skip check in test

			return result
		}

		result.Err = store.CommitContext(dbCtx)

		return result
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestDeleteRole(t *testing.T) {
	store, closeStore := teststore.NewTestStorage(t)

	t.Cleanup(closeStore)

	ctx := context.Background()

	actorID := gidx.PrefixedID("idntusr-abc123")
	roleID := gidx.PrefixedID("permrol-def456")
	roleName := "admins"
	resourceID := gidx.PrefixedID("testten-jkl789")

	dbCtx, err := store.BeginContext(ctx)

	require.NoError(t, err, "no error expected beginning transaction context")

	dbRole, err := store.DeleteRole(dbCtx, roleID)

	require.Error(t, err, "error expected while deleting role which doesn't exist")
	require.ErrorIs(t, err, storage.ErrNoRoleFound, "expected no role found error for missing role")
	assert.Empty(t, dbRole.ID, "expected role to be empty")

	dbCtx, err = store.BeginContext(ctx)

	require.NoError(t, err, "no error expected beginning transaction context")

	createdDBRole, err := store.CreateRole(dbCtx, actorID, roleID, roleName, resourceID)

	require.NoError(t, err, "no error expected while seeding database role")

	err = store.CommitContext(dbCtx)

	require.NoError(t, err, "no error expected while committing role creation")

	dbCtx, err = store.BeginContext(ctx)

	require.NoError(t, err, "no error expected beginning transaction context")

	deletedDBRole, err := store.DeleteRole(dbCtx, roleID)

	require.NoError(t, err, "no error expected while deleting role")

	err = store.CommitContext(dbCtx)

	require.NoError(t, err, "no error expected while committing role deletion")

	role, err := store.GetRoleByID(ctx, roleID)

	require.Error(t, err, "expected error retrieving role")
	assert.ErrorIs(t, err, storage.ErrNoRoleFound, "expected no rows error")
	assert.Empty(t, role.ID, "role id expected to be empty")

	assert.Equal(t, roleID, createdDBRole.ID, "unexpected created role id")
	assert.Equal(t, roleID, deletedDBRole.ID, "unexpected deleted role id")
}
