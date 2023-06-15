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
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/permissions-api/internal/types"
	"go.infratographer.com/x/urnx"
)

func testEngine(ctx context.Context, t *testing.T, namespace string) Engine {
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
	for _, dbType := range []string{"user", "client", "role", "tenant"} {
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

func TestRoles(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace)

	testCases := []testingx.TestCase[[]string, []types.Role]{
		{
			Name: "CreateInvalidAction",
			Input: []string{
				"bad_action",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Role]) {
				assert.Error(t, res.Err)
			},
		},
		{
			Name: "CreateSuccess",
			Input: []string{
				"loadbalancer_get",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Role]) {
				expActions := []string{
					"loadbalancer_get",
				}

				assert.NoError(t, res.Err)
				require.Equal(t, 1, len(res.Success))

				role := res.Success[0]
				assert.Equal(t, expActions, role.Actions)
			},
		},
	}

	testFn := func(ctx context.Context, actions []string) testingx.TestResult[[]types.Role] {
		tenURN, err := urnx.Build(namespace, "tenant", uuid.New())
		require.NoError(t, err)
		tenRes, err := e.NewResourceFromURN(tenURN)
		require.NoError(t, err)

		_, queryToken, err := e.CreateRole(ctx, tenRes, actions)
		if err != nil {
			return testingx.TestResult[[]types.Role]{
				Err: err,
			}
		}

		roles, err := e.ListRoles(ctx, tenRes, queryToken)

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
	e := testEngine(ctx, t, namespace)

	tenURN, err := urnx.Build(namespace, "tenant", uuid.New())
	require.NoError(t, err)
	tenRes, err := e.NewResourceFromURN(tenURN)
	require.NoError(t, err)
	subjURN, err := urnx.Build(namespace, "user", uuid.New())
	require.NoError(t, err)
	subjRes, err := e.NewResourceFromURN(subjURN)
	require.NoError(t, err)
	role, _, err := e.CreateRole(
		ctx,
		tenRes,
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
		queryToken, err := e.AssignSubjectRole(ctx, subjRes, role)
		if err != nil {
			return testingx.TestResult[[]types.Resource]{
				Err: err,
			}
		}

		resources, err := e.ListAssignments(ctx, role, queryToken)

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
	e := testEngine(ctx, t, namespace)

	parentURN, err := urnx.Build(namespace, "tenant", uuid.New())
	require.NoError(t, err)
	parentRes, err := e.NewResourceFromURN(parentURN)
	require.NoError(t, err)
	childURN, err := urnx.Build(namespace, "tenant", uuid.New())
	require.NoError(t, err)
	childRes, err := e.NewResourceFromURN(childURN)
	require.NoError(t, err)

	testCases := []testingx.TestCase[[]types.Relationship, []types.Relationship]{
		{
			Name: "InvalidRelationship",
			Input: []types.Relationship{
				{
					Resource: childRes,
					Relation: "foo",
					Subject:  parentRes,
				},
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Relationship]) {
				assert.ErrorIs(t, errorInvalidRelationship, res.Err)
			},
		},
		{
			Name: "Success",
			Input: []types.Relationship{
				{
					Resource: childRes,
					Relation: "parent",
					Subject:  parentRes,
				},
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
	}

	testFn := func(ctx context.Context, input []types.Relationship) testingx.TestResult[[]types.Relationship] {
		queryToken, err := e.CreateRelationships(ctx, input)
		if err != nil {
			return testingx.TestResult[[]types.Relationship]{
				Err: err,
			}
		}

		rels, err := e.ListRelationships(ctx, childRes, queryToken)

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
	e := testEngine(ctx, t, namespace)

	tenURN, err := urnx.Build(namespace, "tenant", uuid.New())
	require.NoError(t, err)
	tenRes, err := e.NewResourceFromURN(tenURN)
	require.NoError(t, err)
	otherURN, err := urnx.Build(namespace, "tenant", uuid.New())
	require.NoError(t, err)
	otherRes, err := e.NewResourceFromURN(otherURN)
	require.NoError(t, err)
	subjURN, err := urnx.Build(namespace, "user", uuid.New())
	require.NoError(t, err)
	subjRes, err := e.NewResourceFromURN(subjURN)
	require.NoError(t, err)
	role, _, err := e.CreateRole(
		ctx,
		tenRes,
		[]string{
			"loadbalancer_update",
		},
	)
	assert.NoError(t, err)
	queryToken, err := e.AssignSubjectRole(ctx, subjRes, role)
	assert.NoError(t, err)

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
				assert.ErrorIs(t, ErrActionNotAssigned, res.Err)
			},
		},
		{
			Name: "BadAction",
			Input: testInput{
				resource: tenRes,
				action:   "loadbalancer_delete",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[any]) {
				assert.ErrorIs(t, ErrActionNotAssigned, res.Err)
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
		err := e.SubjectHasPermission(ctx, subjRes, input.action, input.resource, queryToken)

		return testingx.TestResult[any]{
			Err: err,
		}
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}
