package query

import (
	"context"
	"testing"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/spicedbx"
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/permissions-api/internal/types"
	"go.infratographer.com/x/gidx"
)

func testEngine(ctx context.Context, t *testing.T, namespace string) Engine {
	config := spicedbx.Config{
		Endpoint: "spicedb:50051",
		Key:      "infradev",
		Insecure: true,
	}

	client, err := spicedbx.NewClient(config, false)
	require.NoError(t, err)

	policy := testPolicy()

	schema, err := spicedbx.GenerateSchema(namespace, policy.Schema())
	require.NoError(t, err)

	request := &pb.WriteSchemaRequest{Schema: schema}
	_, err = client.WriteSchema(ctx, request)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanDB(ctx, t, client, namespace)
	})

	out := NewEngine(namespace, client, WithPolicy(policy))

	return out
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
					TargetTypeNames: []string{
						"tenant",
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
		tenID, err := gidx.NewID("tnntten")
		require.NoError(t, err)
		tenRes, err := e.NewResourceFromID(tenID)
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

	tenID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	tenRes, err := e.NewResourceFromID(tenID)
	require.NoError(t, err)
	subjID, err := gidx.NewID("idntusr")
	require.NoError(t, err)
	subjRes, err := e.NewResourceFromID(subjID)
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
				assert.ErrorIs(t, res.Err, errorInvalidRelationship)
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
		queryToken, err := e.CreateRelationships(ctx, []types.Relationship{input})
		if err != nil {
			return testingx.TestResult[[]types.Relationship]{
				Err: err,
			}
		}

		rels, err := e.ListRelationships(ctx, input.Resource, queryToken)

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
				assert.ErrorIs(t, res.Err, ErrActionNotAssigned)
			},
		},
		{
			Name: "BadAction",
			Input: testInput{
				resource: tenRes,
				action:   "loadbalancer_delete",
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[any]) {
				assert.ErrorIs(t, res.Err, ErrActionNotAssigned)
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
