package iapl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.infratographer.com/permissions-api/internal/testingx"
)

func TestPolicy(t *testing.T) {
	rbac := DefaultRBAC()

	cases := []testingx.TestCase[PolicyDocument, Policy]{
		{
			Name: "TypeExists",
			Input: PolicyDocument{
				ResourceTypes: []ResourceType{
					{
						Name: "foo",
					},
				},
				Unions: []Union{
					{
						Name: "foo",
						ResourceTypeNames: []string{
							"foo",
						},
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.ErrorIs(t, res.Err, ErrorTypeExists)
			},
		},
		{
			Name: "UnknownTypeInUnion",
			Input: PolicyDocument{
				ResourceTypes: []ResourceType{
					{
						Name: "foo",
					},
				},
				Unions: []Union{
					{
						Name: "bar",
						ResourceTypeNames: []string{
							"baz",
						},
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.ErrorIs(t, res.Err, ErrorUnknownType)
			},
		},
		{
			Name: "UnknownTypeInUnion",
			Input: PolicyDocument{
				ResourceTypes: []ResourceType{
					{
						Name: "foo",
					},
				},
				Unions: []Union{
					{
						Name: "bar",
						ResourceTypeNames: []string{
							"baz",
						},
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.ErrorIs(t, res.Err, ErrorUnknownType)
			},
		},
		{
			Name: "UnknownTypeInRelationship",
			Input: PolicyDocument{
				ResourceTypes: []ResourceType{
					{
						Name: "foo",
						Relationships: []Relationship{
							{
								Relation: "bar",
								TargetTypeNames: []string{
									"baz",
								},
							},
						},
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.ErrorIs(t, res.Err, ErrorUnknownType)
			},
		},
		{
			Name: "UnknownActionInCondition",
			Input: PolicyDocument{
				ResourceTypes: []ResourceType{
					{
						Name: "foo",
						Relationships: []Relationship{
							{
								Relation: "bar",
								TargetTypeNames: []string{
									"foo",
								},
							},
						},
					},
				},
				ActionBindings: []ActionBinding{
					{
						TypeName:   "foo",
						ActionName: "qux",
						Conditions: []Condition{
							{
								RoleBinding: &ConditionRoleBinding{},
							},
						},
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.ErrorIs(t, res.Err, ErrorUnknownAction)
			},
		},
		{
			Name: "UnknownActionInCondition",
			Input: PolicyDocument{
				ResourceTypes: []ResourceType{
					{
						Name: "foo",
						Relationships: []Relationship{
							{
								Relation: "bar",
								TargetTypeNames: []string{
									"foo",
								},
							},
						},
					},
				},
				Actions: []Action{
					{
						Name: "qux",
					},
				},
				ActionBindings: []ActionBinding{
					{
						TypeName:   "foo",
						ActionName: "qux",
						Conditions: []Condition{
							{
								RelationshipAction: &ConditionRelationshipAction{
									Relation:   "bar",
									ActionName: "baz",
								},
							},
						},
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.ErrorIs(t, res.Err, ErrorUnknownAction)
			},
		},
		{
			Name: "UnknownRelationInCondition",
			Input: PolicyDocument{
				ResourceTypes: []ResourceType{
					{
						Name: "foo",
					},
				},
				Actions: []Action{
					{
						Name: "qux",
					},
				},
				ActionBindings: []ActionBinding{
					{
						TypeName:   "foo",
						ActionName: "qux",
						Conditions: []Condition{
							{
								RelationshipAction: &ConditionRelationshipAction{
									Relation:   "bar",
									ActionName: "qux",
								},
							},
						},
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.ErrorIs(t, res.Err, ErrorUnknownRelation)
			},
		},
		{
			Name: "UnknownRelationInUnion",
			Input: PolicyDocument{
				ResourceTypes: []ResourceType{
					{
						Name: "foo",
						Relationships: []Relationship{
							{
								Relation: "bar",
								TargetTypeNames: []string{
									"foo",
								},
							},
						},
					},
					{
						Name: "baz",
					},
				},
				Unions: []Union{
					{
						Name: "buzz",
						ResourceTypeNames: []string{
							"foo",
							"baz",
						},
					},
				},
				Actions: []Action{
					{
						Name: "qux",
					},
				},
				ActionBindings: []ActionBinding{
					{
						TypeName:   "buzz",
						ActionName: "qux",
						Conditions: []Condition{
							{
								RelationshipAction: &ConditionRelationshipAction{
									Relation:   "bar",
									ActionName: "qux",
								},
							},
						},
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.ErrorIs(t, res.Err, ErrorUnknownRelation)
			},
		},
		{
			Name: "UnknownActionInUnion",
			Input: PolicyDocument{
				ResourceTypes: []ResourceType{
					{
						Name: "foo",
						Relationships: []Relationship{
							{
								Relation: "bar",
								TargetTypeNames: []string{
									"foo",
								},
							},
						},
					},
					{
						Name: "baz",
						Relationships: []Relationship{
							{
								Relation: "bar",
								TargetTypeNames: []string{
									"foo",
								},
							},
						},
					},
				},
				Unions: []Union{
					{
						Name: "buzz",
						ResourceTypeNames: []string{
							"foo",
							"baz",
						},
					},
				},
				Actions: []Action{
					{
						Name: "qux",
					},
				},
				ActionBindings: []ActionBinding{
					{
						TypeName:   "buzz",
						ActionName: "qux",
						Conditions: []Condition{
							{
								RelationshipAction: &ConditionRelationshipAction{
									Relation:   "bar",
									ActionName: "fizz",
								},
							},
						},
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.ErrorIs(t, res.Err, ErrorUnknownAction)
			},
		},
		{
			Name: "Success",
			Input: PolicyDocument{
				ResourceTypes: []ResourceType{
					{
						Name: "foo",
						Relationships: []Relationship{
							{
								Relation: "bar",
								TargetTypeNames: []string{
									"foo",
								},
							},
						},
					},
					{
						Name: "baz",
						Relationships: []Relationship{
							{
								Relation: "bar",
								TargetTypeNames: []string{
									"foo",
								},
							},
						},
					},
				},
				Unions: []Union{
					{
						Name: "buzz",
						ResourceTypeNames: []string{
							"foo",
							"baz",
						},
					},
				},
				Actions: []Action{
					{
						Name: "qux",
					},
				},
				ActionBindings: []ActionBinding{
					{
						TypeName:   "buzz",
						ActionName: "qux",
						Conditions: []Condition{
							{
								RelationshipAction: &ConditionRelationshipAction{
									Relation:   "bar",
									ActionName: "qux",
								},
							},
						},
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.NoError(t, res.Err)
			},
		},
		{
			Name: "NoRBACProvided",
			Input: PolicyDocument{
				ResourceTypes: []ResourceType{
					{
						Name: "foo",
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.NoError(t, res.Err)
				require.Nil(t, res.Success.RBAC())
			},
		},
		{
			Name: "RoleOwnerMissing",
			Input: PolicyDocument{
				RBAC: &rbac,
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				// unknown resource type: role owner tenant does not exists
				require.ErrorIs(t, res.Err, ErrorUnknownType)
			},
		},
		{
			Name: "RBAC_OK",
			Input: PolicyDocument{
				RBAC: &rbac,
				ResourceTypes: []ResourceType{
					{
						Name: "tenant",
					},
				},
			},
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[Policy]) {
				require.NoError(t, res.Err)
				require.NotNil(t, res.Success.RBAC())
			},
		},
	}

	testFn := func(_ context.Context, doc PolicyDocument) testingx.TestResult[Policy] {
		p := NewPolicy(doc)
		err := p.Validate()

		return testingx.TestResult[Policy]{
			Success: p,
			Err:     err,
		}
	}

	testingx.RunTests(context.Background(), t, cases, testFn)
}
