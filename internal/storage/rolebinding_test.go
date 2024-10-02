package storage_test

import (
	"context"
	"testing"

	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/storage/teststore"
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/permissions-api/internal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.infratographer.com/x/gidx"
)

func TestGetRoleBindingByID(t *testing.T) {
	store, closeStore := teststore.NewTestStorage(t)
	t.Cleanup(closeStore)

	ctx := context.Background()
	actorID := gidx.PrefixedID("idntusr-user")
	resourceID := gidx.PrefixedID("tentten-tenant")
	rbID := gidx.MustNewID("permrbn")

	dbCtx, err := store.BeginContext(ctx)
	require.NoError(t, err, "no error expected beginning transaction context")

	rb, err := store.CreateRoleBinding(dbCtx, actorID, rbID, resourceID, t.Name())
	require.NoError(t, err, "no error expected creating role binding")

	err = store.CommitContext(dbCtx)
	require.NoError(t, err, "no error expected committing transaction context")

	tc := []testingx.TestCase[gidx.PrefixedID, types.RoleBinding]{
		{
			Name:  "NotFound",
			Input: "permrbn-definitely_not_exists",
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				require.ErrorIs(t, res.Err, storage.ErrRoleBindingNotFound, "expected error to be role binding not found")
				assert.ErrorIs(t, res.Err, storage.ErrRoleBindingNotFound)
				require.Empty(t, res.Success.ID)
			},
		},
		{
			Name:  "ok",
			Input: rbID,
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				require.NoError(t, res.Err, "no error expected")

				assert.Equal(t, rb.ID, res.Success.ID)
				assert.Equal(t, rb.CreatedAt, res.Success.CreatedAt)
				assert.Equal(t, rb.UpdatedAt, res.Success.UpdatedAt)
				assert.Equal(t, rb.CreatedBy, res.Success.CreatedBy)
				assert.Equal(t, rb.UpdatedBy, res.Success.UpdatedBy)
			},
		},
	}

	testfn := func(ctx context.Context, input gidx.PrefixedID) testingx.TestResult[types.RoleBinding] {
		rb, err := store.GetRoleBindingByID(ctx, input)

		return testingx.TestResult[types.RoleBinding]{Success: rb, Err: err}
	}

	testingx.RunTests(ctx, t, tc, testfn)
}

func TestListResourceRoleBindings(t *testing.T) {
	store, closeStore := teststore.NewTestStorage(t)
	t.Cleanup(closeStore)

	ctx := context.Background()
	actorID := gidx.PrefixedID("idntusr-user")
	resourceID := gidx.PrefixedID("tentten-tenant")

	rbIDs := []gidx.PrefixedID{
		gidx.MustNewID("permrbn"),
		gidx.MustNewID("permrbn"),
	}

	rbs := map[gidx.PrefixedID]types.RoleBinding{}

	dbCtx, err := store.BeginContext(ctx)
	require.NoError(t, err, "no error expected beginning transaction context")

	for _, rbID := range rbIDs {
		rbs[rbID], err = store.CreateRoleBinding(dbCtx, actorID, rbID, resourceID, t.Name())
		require.NoError(t, err, "no error expected creating role binding")
	}

	err = store.CommitContext(dbCtx)
	require.NoError(t, err, "no error expected committing transaction context")

	tc := []testingx.TestCase[gidx.PrefixedID, []types.RoleBinding]{
		{
			Name:  "NotFound",
			Input: "tentten-definitely_not_exists",
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[[]types.RoleBinding]) {
				assert.NoError(t, res.Err, "no error expected")
				assert.Len(t, res.Success, 0, "an empty list is expected")
			},
		},
		{
			Name:  "ok",
			Input: resourceID,
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[[]types.RoleBinding]) {
				assert.NoError(t, res.Err, "no error expected")
				assert.Len(t, res.Success, len(rbs), "expected number of role bindings")

				for _, rb := range res.Success {
					assert.Equal(t, rb.ID, rbs[rb.ID].ID)
					assert.Equal(t, rb.CreatedAt, rbs[rb.ID].CreatedAt)
					assert.Equal(t, rb.UpdatedAt, rbs[rb.ID].UpdatedAt)
					assert.Equal(t, rb.CreatedBy, rbs[rb.ID].CreatedBy)
					assert.Equal(t, rb.UpdatedBy, rbs[rb.ID].UpdatedBy)
				}
			},
		},
	}

	testfn := func(ctx context.Context, input gidx.PrefixedID) testingx.TestResult[[]types.RoleBinding] {
		rb, err := store.ListResourceRoleBindings(ctx, input)

		return testingx.TestResult[[]types.RoleBinding]{Success: rb, Err: err}
	}

	testingx.RunTests(ctx, t, tc, testfn)
}

func TestCreateRoleBinding(t *testing.T) {
	store, closeStore := teststore.NewTestStorage(t)
	t.Cleanup(closeStore)

	ctx := context.Background()
	actorID := gidx.PrefixedID("idntusr-user")
	resourceID := gidx.PrefixedID("tentten-tenant")
	rbID := gidx.MustNewID("permrbn")

	tc := []testingx.TestCase[gidx.PrefixedID, types.RoleBinding]{
		{
			Name:  "ok",
			Input: rbID,
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				require.NoError(t, res.Err, "no error expected")

				assert.Equal(t, rbID, res.Success.ID)
				assert.NotZero(t, res.Success.CreatedAt, "expected created at to be set")
				assert.NotZero(t, res.Success.UpdatedAt, "expected updated at to be set")
				assert.Equal(t, actorID, res.Success.CreatedBy)
				assert.Equal(t, actorID, res.Success.UpdatedBy)
			},
			Sync: true,
		},
		{
			Name:  "IDConflict",
			Input: rbID,
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.Error(t, res.Err)
				require.Empty(t, res.Success.ID)
			},
			Sync: true,
		},
	}

	testfn := func(ctx context.Context, input gidx.PrefixedID) testingx.TestResult[types.RoleBinding] {
		result := testingx.TestResult[types.RoleBinding]{}

		dbCtx, err := store.BeginContext(ctx)
		if err != nil {
			result.Err = err

			return result
		}

		result.Success, result.Err = store.CreateRoleBinding(dbCtx, actorID, input, resourceID, t.Name())
		if result.Err != nil {
			store.RollbackContext(dbCtx) //nolint:errcheck // skip check in test

			return result
		}

		result.Err = store.CommitContext(dbCtx)

		return result
	}

	testingx.RunTests(ctx, t, tc, testfn)
}

func TestUpdateRoleBinding(t *testing.T) {
	store, closeStore := teststore.NewTestStorage(t)
	t.Cleanup(closeStore)

	ctx := context.Background()
	actorID := gidx.PrefixedID("idntusr-user")
	theOtherGuy := gidx.PrefixedID("idntusr-the_other_guy")
	resourceID := gidx.PrefixedID("tentten-tenant")
	rbID := gidx.MustNewID("permrbn")

	dbCtx, err := store.BeginContext(ctx)
	require.NoError(t, err, "no error expected beginning transaction context")

	_, err = store.CreateRoleBinding(dbCtx, actorID, rbID, resourceID, t.Name())
	require.NoError(t, err, "no error expected creating role binding")

	err = store.CommitContext(dbCtx)
	require.NoError(t, err, "no error expected committing transaction context")

	tc := []testingx.TestCase[gidx.PrefixedID, types.RoleBinding]{
		{
			Name:  "ok",
			Input: rbID,
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				require.NoError(t, res.Err, "no error expected")

				assert.Equal(t, rbID, res.Success.ID)
				assert.NotZero(t, res.Success.CreatedAt, "expected created at to be set")
				assert.NotZero(t, res.Success.UpdatedAt, "expected updated at to be set")
				assert.Equal(t, actorID, res.Success.CreatedBy)
				assert.Equal(t, theOtherGuy, res.Success.UpdatedBy)
			},
		},
		{
			Name:  "NotFound",
			Input: "permrbn-definitely_not_exists",
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.ErrorIs(t, res.Err, storage.ErrRoleBindingNotFound)
				require.Empty(t, res.Success.ID)
			},
		},
	}

	testfn := func(ctx context.Context, input gidx.PrefixedID) testingx.TestResult[types.RoleBinding] {
		result := testingx.TestResult[types.RoleBinding]{}

		dbCtx, err := store.BeginContext(ctx)
		if err != nil {
			result.Err = err

			return result
		}

		result.Success, result.Err = store.UpdateRoleBinding(dbCtx, theOtherGuy, input)
		if result.Err != nil {
			store.RollbackContext(dbCtx) //nolint:errcheck // skip check in
			return result
		}

		result.Err = store.CommitContext(dbCtx)

		return result
	}

	testingx.RunTests(ctx, t, tc, testfn)
}

func TestDeleteRoleBinding(t *testing.T) {
	store, closeStore := teststore.NewTestStorage(t)
	t.Cleanup(closeStore)

	ctx := context.Background()
	actorID := gidx.PrefixedID("idntusr-user")
	resourceID := gidx.PrefixedID("tentten-tenant")
	rbID := gidx.MustNewID("permrbn")

	dbCtx, err := store.BeginContext(ctx)
	require.NoError(t, err, "no error expected beginning transaction context")

	_, err = store.CreateRoleBinding(dbCtx, actorID, rbID, resourceID, t.Name())
	require.NoError(t, err, "no error expected creating role binding")

	err = store.CommitContext(dbCtx)
	require.NoError(t, err, "no error expected committing transaction context")

	tc := []testingx.TestCase[gidx.PrefixedID, error]{
		{
			Name:  "ok",
			Input: rbID,
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[error]) {
				assert.NoError(t, res.Err, "no error expected")
			},
		},
		{
			Name:  "NotFound",
			Input: "permrbn-definitely_not_exists",
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[error]) {
				assert.ErrorIs(t, res.Err, storage.ErrRoleBindingNotFound)
			},
		},
	}

	testfn := func(ctx context.Context, input gidx.PrefixedID) testingx.TestResult[error] {
		result := testingx.TestResult[error]{}

		dbCtx, err := store.BeginContext(ctx)
		if err != nil {
			result.Err = err

			return result
		}

		result.Err = store.DeleteRoleBinding(dbCtx, input)
		if result.Err != nil {
			store.RollbackContext(dbCtx) //nolint:errcheck // skip check in test

			return result
		}

		result.Err = store.CommitContext(dbCtx)

		return result
	}

	testingx.RunTests(ctx, t, tc, testfn)
}
