package query

import (
	"context"
	"testing"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.infratographer.com/x/gidx"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/spicedbx"
	"go.infratographer.com/permissions-api/internal/storage/teststore"
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/permissions-api/internal/types"
)

func testEngine(ctx context.Context, t *testing.T, namespace string, policy iapl.Policy) *engine {
	config := spicedbx.Config{
		Endpoint: "spicedb:50051",
		Key:      "infradev",
		Insecure: true,
	}

	client, err := spicedbx.NewClient(config, false)
	require.NoError(t, err)

	store, cleanStore := teststore.NewTestStorage(t)

	schema, err := spicedbx.GenerateSchema(namespace, policy.Schema())
	require.NoError(t, err)

	request := &pb.WriteSchemaRequest{Schema: schema}
	_, err = client.WriteSchema(ctx, request)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanDB(ctx, t, client, namespace, policy)
		cleanStore()
	})

	// We call the constructor here to ensure the engine is created appropriately, but
	// then return the underlying type so we can do testing with it.
	out, err := NewEngine(namespace, client, store, WithPolicy(policy))
	require.NoError(t, err)

	return out.(*engine)
}

func testPolicy() iapl.Policy {
	policyDocument := iapl.DefaultPolicyDocument()

	policyDocument.ResourceTypes = append(policyDocument.ResourceTypes,
		iapl.ResourceType{
			Name:     "child",
			IDPrefix: "chldten",
			Relationships: []iapl.Relationship{
				{
					Relation: "parent",
					TargetTypes: []types.TargetType{
						{Name: "tenant"},
					},
				},
			},
		},
	)

	policy := iapl.NewPolicy(policyDocument)
	if err := policy.Validate(); err != nil {
		panic(err)
	}

	return policy
}

func cleanDB(ctx context.Context, t *testing.T, client *authzed.Client, namespace string, p iapl.Policy) {
	for _, resourceType := range p.Schema() {
		dbType := resourceType.Name
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

func TestCreateRoles(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, testPolicy())

	testCases := []testingx.TestCase[[]string, types.Role]{
		{
			Name: "CreateInvalidAction",
			Input: []string{
				"bad_action",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				assert.Error(t, res.Err)
			},
		},
		{
			Name:  "CreateNoActions",
			Input: []string{},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				expActions := []string{}

				require.NoError(t, res.Err)

				role := res.Success
				assert.Equal(t, expActions, role.Actions)
			},
		},
		{
			Name: "CreateSuccess",
			Input: []string{
				"loadbalancer_get",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				expActions := []string{
					"loadbalancer_get",
				}

				require.NoError(t, res.Err)

				role := res.Success
				assert.Equal(t, expActions, role.Actions)
			},
		},
	}

	testFn := func(ctx context.Context, actions []string) testingx.TestResult[types.Role] {
		tenID, err := gidx.NewID("tnntten")
		require.NoError(t, err)
		tenRes, err := e.NewResourceFromID(tenID)
		require.NoError(t, err)
		actorRes, err := e.NewResourceFromID(gidx.MustNewID("idntusr"))
		require.NoError(t, err)

		role, err := e.CreateRole(ctx, actorRes, tenRes, "test", actions)
		if err != nil {
			return testingx.TestResult[types.Role]{
				Err: err,
			}
		}

		roleResource, err := e.NewResourceFromID(role.ID)
		require.NoError(t, err)

		obs, err := e.GetRole(ctx, roleResource)

		return testingx.TestResult[types.Role]{
			Success: obs,
			Err:     err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestGetRoles(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, testPolicy())
	tenID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	tenRes, err := e.NewResourceFromID(tenID)
	require.NoError(t, err)
	actorRes, err := e.NewResourceFromID(gidx.MustNewID("idntusr"))
	require.NoError(t, err)

	role, err := e.CreateRole(ctx, actorRes, tenRes, "test", []string{"loadbalancer_get"})
	require.NoError(t, err)
	roleRes, err := e.NewResourceFromID(role.ID)
	require.NoError(t, err)

	missingRes, err := e.NewResourceFromID(gidx.PrefixedID("permrol-notfound"))
	require.NoError(t, err)

	testCases := []testingx.TestCase[types.Resource, types.Role]{
		{
			Name:  "GetRoleNotFound",
			Input: missingRes,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				assert.ErrorIs(t, res.Err, ErrRoleNotFound)
			},
		},
		{
			Name:  "GetSuccess",
			Input: roleRes,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				expActions := []string{
					"loadbalancer_get",
				}

				assert.NoError(t, res.Err)
				require.NotEmpty(t, res.Success.ID)

				assert.Equal(t, expActions, res.Success.Actions)
			},
		},
	}

	testFn := func(ctx context.Context, roleResource types.Resource) testingx.TestResult[types.Role] {
		roles, err := e.GetRole(ctx, roleResource)

		return testingx.TestResult[types.Role]{
			Success: roles,
			Err:     err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestRoleUpdate(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, testPolicy())

	tenID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	tenRes, err := e.NewResourceFromID(tenID)
	require.NoError(t, err)
	actorRes, err := e.NewResourceFromID(gidx.MustNewID("idntusr"))
	require.NoError(t, err)
	actorUpdateRes, err := e.NewResourceFromID(gidx.MustNewID("idntusr"))
	require.NoError(t, err)

	role, err := e.CreateRole(ctx, actorRes, tenRes, "test", []string{"loadbalancer_get"})
	require.NoError(t, err)
	roles, err := e.ListRoles(ctx, tenRes)
	require.NoError(t, err)
	require.NotEmpty(t, roles)

	testCases := []testingx.TestCase[gidx.PrefixedID, types.Role]{
		{
			Name:  "UpdateMissingRole",
			Input: gidx.MustNewID(RolePrefix),
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				require.Error(t, res.Err)
				assert.ErrorIs(t, res.Err, ErrRoleNotFound)
			},
		},
		{
			Name:  "UpdateSuccess",
			Input: role.ID,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				require.NoError(t, res.Err)
				assert.Equal(t, "test2", res.Success.Name)
				assert.Equal(t, role.Actions, res.Success.Actions)
				assert.Equal(t, role.CreatedBy, res.Success.CreatedBy)
				assert.Equal(t, actorUpdateRes.ID, res.Success.UpdatedBy)
				assert.Equal(t, role.CreatedAt, res.Success.CreatedAt)
				assert.NotEqual(t, role.UpdatedAt, res.Success.UpdatedAt)
			},
		},
	}

	testFn := func(ctx context.Context, roleID gidx.PrefixedID) testingx.TestResult[types.Role] {
		roleResource, err := e.NewResourceFromID(roleID)
		if err != nil {
			return testingx.TestResult[types.Role]{
				Err: err,
			}
		}

		_, err = e.UpdateRole(ctx, actorUpdateRes, roleResource, "test2", nil)
		if err != nil {
			return testingx.TestResult[types.Role]{
				Err: err,
			}
		}

		updatedRole, err := e.GetRole(ctx, roleResource)

		return testingx.TestResult[types.Role]{
			Success: updatedRole,
			Err:     err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestListRoles(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, testPolicy())

	actorRes, err := e.NewResourceFromID(gidx.MustNewID("idntusr"))
	require.NoError(t, err)

	type (
		tenCtxKey  struct{}
		roleCtxKey struct{}
	)

	var (
		tenCtx  tenCtxKey
		roleCtx roleCtxKey
	)

	testCases := []testingx.TestCase[any, []types.Role]{
		{
			Name: "RoleFoundWithActions",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				tenID, err := gidx.NewID("tnntten")
				require.NoError(t, err)

				tenRes, err := e.NewResourceFromID(tenID)
				require.NoError(t, err)

				role, err := e.CreateRole(ctx, actorRes, tenRes, t.Name(), []string{"loadbalancer_get"})
				require.NoError(t, err)
				require.NotEmpty(t, role.ID)

				ctx = context.WithValue(ctx, tenCtx, tenRes)
				ctx = context.WithValue(ctx, roleCtx, role)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Role]) {
				assert.NoError(t, res.Err)
				require.NotEmpty(t, res.Success)
				assert.Equal(t, ctx.Value(roleCtx), res.Success[0])
				assert.NotEmpty(t, res.Success[0].Actions)
			},
		},
		{
			Name: "RoleFoundWithoutActions",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				tenID, err := gidx.NewID("tnntten")
				require.NoError(t, err)

				tenRes, err := e.NewResourceFromID(tenID)
				require.NoError(t, err)

				role, err := e.CreateRole(ctx, actorRes, tenRes, t.Name(), nil)
				require.NoError(t, err)
				require.NotEmpty(t, role.ID)

				ctx = context.WithValue(ctx, tenCtx, tenRes)
				ctx = context.WithValue(ctx, roleCtx, role)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Role]) {
				assert.NoError(t, res.Err)
				require.NotEmpty(t, res.Success)
				assert.Equal(t, ctx.Value(roleCtx), res.Success[0])
				assert.Empty(t, res.Success[0].Actions)
			},
		},
	}

	testFn := func(ctx context.Context, _ any) testingx.TestResult[[]types.Role] {
		tenRes := ctx.Value(tenCtx).(types.Resource)

		roles, err := e.ListRoles(ctx, tenRes)

		return testingx.TestResult[[]types.Role]{
			Success: roles,
			Err:     err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestRoleDelete(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, testPolicy())

	tenID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	tenRes, err := e.NewResourceFromID(tenID)
	require.NoError(t, err)
	actorRes, err := e.NewResourceFromID(gidx.MustNewID("idntusr"))
	require.NoError(t, err)

	role, err := e.CreateRole(ctx, actorRes, tenRes, "test", []string{"loadbalancer_get"})
	require.NoError(t, err)
	roles, err := e.ListRoles(ctx, tenRes)
	require.NoError(t, err)
	require.NotEmpty(t, roles)

	testCases := []testingx.TestCase[gidx.PrefixedID, []types.Role]{
		{
			Name:  "DeleteMissingRole",
			Input: gidx.MustNewID(RolePrefix),
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Role]) {
				assert.Error(t, res.Err)
			},
		},
		{
			Name:  "DeleteSuccess",
			Input: role.ID,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Role]) {
				assert.NoError(t, res.Err)
				require.Empty(t, res.Success)
			},
		},
	}

	testFn := func(ctx context.Context, roleID gidx.PrefixedID) testingx.TestResult[[]types.Role] {
		roleResource, err := e.NewResourceFromID(roleID)
		if err != nil {
			return testingx.TestResult[[]types.Role]{
				Err: err,
			}
		}

		err = e.DeleteRole(ctx, roleResource)
		if err != nil {
			return testingx.TestResult[[]types.Role]{
				Err: err,
			}
		}

		roles, err := e.ListRoles(ctx, tenRes)

		return testingx.TestResult[[]types.Role]{
			Success: roles,
			Err:     err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestAssignments(t *testing.T) {
	namespace := "testassignments"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, testPolicy())

	tenID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	tenRes, err := e.NewResourceFromID(tenID)
	require.NoError(t, err)
	subjID, err := gidx.NewID("idntusr")
	require.NoError(t, err)
	subjRes, err := e.NewResourceFromID(subjID)
	require.NoError(t, err)
	actorRes, err := e.NewResourceFromID(gidx.MustNewID("idntusr"))
	require.NoError(t, err)
	role, err := e.CreateRole(
		ctx,
		actorRes,
		tenRes,
		"test",
		[]string{
			"loadbalancer_update",
		},
	)
	assert.NoError(t, err)

	testCases := []testingx.TestCase[types.Role, []types.Resource]{
		{
			Name:  "Success",
			Input: role,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Resource]) {
				expAssignments := []types.Resource{
					subjRes,
				}

				assert.NoError(t, res.Err)
				assert.Equal(t, expAssignments, res.Success)
			},
		},
	}

	testFn := func(ctx context.Context, role types.Role) testingx.TestResult[[]types.Resource] {
		err := e.AssignSubjectRole(ctx, subjRes, role)
		if err != nil {
			return testingx.TestResult[[]types.Resource]{
				Err: err,
			}
		}

		resources, err := e.ListAssignments(ctx, role)

		return testingx.TestResult[[]types.Resource]{
			Success: resources,
			Err:     err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestUnassignments(t *testing.T) {
	namespace := "testassignments"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, testPolicy())

	tenID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	tenRes, err := e.NewResourceFromID(tenID)
	require.NoError(t, err)
	subjID, err := gidx.NewID("idntusr")
	require.NoError(t, err)
	subjRes, err := e.NewResourceFromID(subjID)
	require.NoError(t, err)
	actorRes, err := e.NewResourceFromID(gidx.MustNewID("idntusr"))
	require.NoError(t, err)
	role, err := e.CreateRole(
		ctx,
		actorRes,
		tenRes,
		"test",
		[]string{
			"loadbalancer_update",
		},
	)
	assert.NoError(t, err)

	testCases := []testingx.TestCase[types.Role, []types.Resource]{
		{
			Name:  "Success",
			Input: role,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Resource]) {
				assert.NoError(t, res.Err)
				assert.Empty(t, res.Success)
			},
		},
	}

	testFn := func(ctx context.Context, role types.Role) testingx.TestResult[[]types.Resource] {
		err := e.AssignSubjectRole(ctx, subjRes, role)
		if err != nil {
			return testingx.TestResult[[]types.Resource]{
				Err: err,
			}
		}

		resources, err := e.ListAssignments(ctx, role)
		if err != nil {
			return testingx.TestResult[[]types.Resource]{
				Err: err,
			}
		}

		var found bool

		for _, resource := range resources {
			if resource.ID == subjRes.ID {
				found = true

				break
			}
		}

		require.True(t, found, "expected assignment to be found")

		err = e.UnassignSubjectRole(ctx, subjRes, role)
		if err != nil {
			return testingx.TestResult[[]types.Resource]{
				Err: err,
			}
		}

		resources, err = e.ListAssignments(ctx, role)

		return testingx.TestResult[[]types.Resource]{
			Success: resources,
			Err:     err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestRelationships(t *testing.T) {
	namespace := "testrelationships"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, testPolicy())

	parentID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	parentRes, err := e.NewResourceFromID(parentID)
	require.NoError(t, err)
	childID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	childRes, err := e.NewResourceFromID(childID)
	require.NoError(t, err)
	child2ID, err := gidx.NewID("chldten")
	require.NoError(t, err)
	child2Res, err := e.NewResourceFromID(child2ID)
	require.NoError(t, err)

	testCases := []testingx.TestCase[types.Relationship, []types.Relationship]{
		{
			Name: "InvalidRelationship",
			Input: types.Relationship{
				Resource: childRes,
				Relation: "foo",
				Subject:  parentRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Relationship]) {
				assert.ErrorIs(t, res.Err, ErrInvalidRelationship)
			},
		},
		{
			Name: "Success",
			Input: types.Relationship{
				Resource: childRes,
				Relation: "parent",
				Subject:  parentRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Relationship]) {
				expRels := []types.Relationship{
					{
						Resource: childRes,
						Relation: "parent",
						Subject:  parentRes,
					},
				}

				require.NoError(t, res.Err)
				assert.Equal(t, expRels, res.Success)
			},
		},
		{
			Name: "Different Success",
			Input: types.Relationship{
				Resource: child2Res,
				Relation: "parent",
				Subject:  parentRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Relationship]) {
				expRels := []types.Relationship{
					{
						Resource: child2Res,
						Relation: "parent",
						Subject:  parentRes,
					},
				}

				require.NoError(t, res.Err)
				assert.Equal(t, expRels, res.Success)
			},
		},
	}

	testFn := func(ctx context.Context, input types.Relationship) testingx.TestResult[[]types.Relationship] {
		err := e.CreateRelationships(ctx, []types.Relationship{input})
		if err != nil {
			return testingx.TestResult[[]types.Relationship]{
				Err: err,
			}
		}

		rels, err := e.ListRelationshipsFrom(ctx, input.Resource)

		return testingx.TestResult[[]types.Relationship]{
			Success: rels,
			Err:     err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestRelationshipDelete(t *testing.T) {
	namespace := "testrelationships"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, testPolicy())

	parentID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	parentRes, err := e.NewResourceFromID(parentID)
	require.NoError(t, err)
	childID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	childRes, err := e.NewResourceFromID(childID)
	require.NoError(t, err)

	relReq := types.Relationship{
		Resource: childRes,
		Relation: "parent",
		Subject:  parentRes,
	}

	err = e.CreateRelationships(ctx, []types.Relationship{relReq})
	require.NoError(t, err)

	createdResources, err := e.ListRelationshipsFrom(ctx, childRes)
	require.NoError(t, err)
	require.NotEmpty(t, createdResources)

	testCases := []testingx.TestCase[types.Relationship, []types.Relationship]{
		{
			Name: "InvalidRelationship",
			Input: types.Relationship{
				Resource: childRes,
				Relation: "foo",
				Subject:  parentRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Relationship]) {
				assert.ErrorIs(t, res.Err, ErrInvalidRelationship)
			},
		},
		{
			Name: "Success",
			Input: types.Relationship{
				Resource: childRes,
				Relation: "parent",
				Subject:  parentRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Relationship]) {
				require.NoError(t, res.Err)
				assert.Empty(t, res.Success)
			},
		},
	}

	testFn := func(ctx context.Context, input types.Relationship) testingx.TestResult[[]types.Relationship] {
		err := e.DeleteRelationships(ctx, input)
		if err != nil {
			return testingx.TestResult[[]types.Relationship]{
				Err: err,
			}
		}

		rels, err := e.ListRelationshipsFrom(ctx, input.Resource)

		return testingx.TestResult[[]types.Relationship]{
			Success: rels,
			Err:     err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestSubjectActions(t *testing.T) {
	namespace := "infratestactions"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, testPolicy())

	parentID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	parentRes, err := e.NewResourceFromID(parentID)
	require.NoError(t, err)
	tenID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	tenRes, err := e.NewResourceFromID(tenID)
	require.NoError(t, err)
	otherID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	otherRes, err := e.NewResourceFromID(otherID)
	require.NoError(t, err)
	subjID, err := gidx.NewID("idntusr")
	require.NoError(t, err)
	subjRes, err := e.NewResourceFromID(subjID)
	require.NoError(t, err)
	actorRes, err := e.NewResourceFromID(gidx.MustNewID("idntusr"))
	require.NoError(t, err)
	role, err := e.CreateRole(
		ctx,
		actorRes,
		tenRes,
		"test",
		[]string{
			"loadbalancer_update",
		},
	)
	assert.NoError(t, err)
	err = e.AssignSubjectRole(ctx, subjRes, role)
	assert.NoError(t, err)

	// Force a ZedToken to be created for the relevant resources. This creates a hierarchy where
	// the tenRes tenant and otherRes tenant are both child resources of the parentRes tenant.
	rels := []types.Relationship{
		{
			Resource: tenRes,
			Relation: "parent",
			Subject:  parentRes,
		},
		{
			Resource: otherRes,
			Relation: "parent",
			Subject:  parentRes,
		},
	}
	err = e.CreateRelationships(ctx, rels)
	require.NoError(t, err)

	type testInput struct {
		resource types.Resource
		action   string
	}

	testCases := []testingx.TestCase[testInput, any]{
		{
			Name: "BadResource",
			Input: testInput{
				resource: otherRes,
				action:   "loadbalancer_update",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[any]) {
				assert.ErrorIs(t, res.Err, ErrActionNotAssigned)
			},
		},
		{
			Name: "DeniedAction",
			Input: testInput{
				resource: tenRes,
				action:   "loadbalancer_delete",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[any]) {
				assert.ErrorIs(t, res.Err, ErrActionNotAssigned)
			},
		},
		{
			Name: "BadAction",
			Input: testInput{
				resource: tenRes,
				action:   "bad_action",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[any]) {
				assert.ErrorIs(t, res.Err, ErrInvalidAction)
			},
		},
		{
			Name: "Success",
			Input: testInput{
				resource: tenRes,
				action:   "loadbalancer_update",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[any]) {
				assert.NoError(t, res.Err)
			},
		},
	}

	testFn := func(ctx context.Context, input testInput) testingx.TestResult[any] {
		err := e.SubjectHasPermission(ctx, subjRes, input.action, input.resource)

		return testingx.TestResult[any]{
			Err: err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}
