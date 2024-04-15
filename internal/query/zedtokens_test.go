package query

import (
	"context"
	"testing"

	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/permissions-api/internal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.infratographer.com/x/gidx"
)

func TestConsistency(t *testing.T) {
	namespace := "testconsistency"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, testPolicy())

	tenantID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	tenantRes, err := e.NewResourceFromID(tenantID)
	require.NoError(t, err)

	parentID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	parentRes, err := e.NewResourceFromID(parentID)
	require.NoError(t, err)

	otherID, err := gidx.NewID("tnntten")
	require.NoError(t, err)
	otherRes, err := e.NewResourceFromID(otherID)
	require.NoError(t, err)

	testCases := []testingx.TestCase[types.Resource, string]{
		{
			Name:  "WithZedToken",
			Input: tenantRes,
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				rels := []types.Relationship{
					{
						Resource: tenantRes,
						Relation: "parent",
						Subject:  parentRes,
					},
				}

				err := e.CreateRelationships(ctx, rels)

				require.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[string]) {
				assert.NoError(t, res.Err)
				assert.Equal(t, consistencyAtLeastAsFresh, res.Success)
			},
		},
		{
			Name:  "WithoutZedToken",
			Input: otherRes,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[string]) {
				assert.NoError(t, res.Err)
				assert.Equal(t, consistencyMinimizeLatency, res.Success)
			},
		},
	}

	testFn := func(ctx context.Context, res types.Resource) testingx.TestResult[string] {
		_, consistencyName := e.determineConsistency(ctx, res)

		out := testingx.TestResult[string]{
			Success: consistencyName,
		}

		return out
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}
