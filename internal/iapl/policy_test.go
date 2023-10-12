package iapl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.infratographer.com/permissions-api/internal/testingx"
)

func TestPolicy(t *testing.T) {
	cases := []testingx.TestCase[PolicyDocument, struct{}]{
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
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[struct{}]) {
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
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[struct{}]) {
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
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[struct{}]) {
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
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[struct{}]) {
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
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[struct{}]) {
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
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[struct{}]) {
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
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[struct{}]) {
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
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[struct{}]) {
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
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[struct{}]) {
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
			CheckFn: func(_ context.Context, t *testing.T, res testingx.TestResult[struct{}]) {
				require.NoError(t, res.Err)
			},
		},
	}

	testFn := func(_ context.Context, p PolicyDocument) testingx.TestResult[struct{}] {
		policy := NewPolicy(p)
		err := policy.Validate()

		return testingx.TestResult[struct{}]{
			Success: struct{}{},
			Err:     err,
		}
	}

	testingx.RunTests(context.Background(), t, cases, testFn)
}
