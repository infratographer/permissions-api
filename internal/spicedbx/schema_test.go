package spicedbx

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.infratographer.com/permissions-api/internal/types"
)

func TestSchema(t *testing.T) {
	t.Parallel()

	type testInput struct {
		namespace     string
		resourceTypes []types.ResourceType
	}

	type testResult struct {
		success string
		err     error
	}

	type testCase struct {
		name    string
		input   testInput
		checkFn func(*testing.T, testResult)
	}

	resourceTypes := []types.ResourceType{
		{
			Name: "user",
		},
		{
			Name: "client",
		},
		{
			Name: "role",
			Relationships: []types.ResourceTypeRelationship{
				{
					Relation: "subject",
					Types: []types.TargetType{
						{Name: "user"},
						{Name: "client"},
					},
				},
			},
		},
		{
			Name: "tenant",
			Relationships: []types.ResourceTypeRelationship{
				{
					Relation: "parent",
					Types: []types.TargetType{
						{Name: "tenant"},
					},
				},
				{
					Relation: "loadbalancer_create_rel",
					Types: []types.TargetType{
						{Name: "role", SubjectRelation: "subject"},
					},
				},
				{
					Relation: "loadbalancer_get_rel",
					Types: []types.TargetType{
						{Name: "role", SubjectRelation: "subject"},
					},
				},
				{
					Relation: "port_create_rel",
					Types: []types.TargetType{
						{Name: "role", SubjectRelation: "subject"},
					},
				},
				{
					Relation: "port_get_rel",
					Types: []types.TargetType{
						{Name: "role", SubjectRelation: "subject"},
					},
				},
			},
			Actions: []types.Action{
				{
					Name: "loadbalancer_create",
					Conditions: []types.Condition{
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation: "loadbalancer_create_rel",
							},
							RoleBinding: &types.ConditionRoleBinding{},
						},
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation:   "parent",
								ActionName: "loadbalancer_create",
							},
						},
					},
				},
				{
					Name: "loadbalancer_get",
					Conditions: []types.Condition{
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation: "loadbalancer_get_rel",
							},
							RoleBinding: &types.ConditionRoleBinding{},
						},
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation:   "parent",
								ActionName: "loadbalancer_get",
							},
						},
					},
				},
				{
					Name: "port_create",
					Conditions: []types.Condition{
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation: "port_create_rel",
							},
							RoleBinding: &types.ConditionRoleBinding{},
						},
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation:   "parent",
								ActionName: "port_create",
							},
						},
					},
				},
				{
					Name: "port_get",
					Conditions: []types.Condition{
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation: "port_get_rel",
							},
							RoleBinding: &types.ConditionRoleBinding{},
						},
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation:   "parent",
								ActionName: "port_get",
							},
						},
					},
				},
			},
		},
		{
			Name: "loadbalancer",
			Relationships: []types.ResourceTypeRelationship{
				{
					Relation: "owner",
					Types: []types.TargetType{
						{Name: "tenant"},
					},
				},
				{
					Relation: "loadbalancer_get_rel",
					Types: []types.TargetType{
						{Name: "role", SubjectRelation: "subject"},
					},
				},
			},
			Actions: []types.Action{
				{
					Name: "loadbalancer_get",
					Conditions: []types.Condition{
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation: "loadbalancer_get_rel",
							},
							RoleBinding: &types.ConditionRoleBinding{},
						},
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation:   "owner",
								ActionName: "loadbalancer_get",
							},
						},
					},
				},
			},
		},
		{
			Name: "port",
			Relationships: []types.ResourceTypeRelationship{
				{
					Relation: "owner",
					Types: []types.TargetType{
						{Name: "tenant"},
					},
				},
				{
					Relation: "port_get_rel",
					Types: []types.TargetType{
						{Name: "role", SubjectRelation: "subject"},
					},
				},
			},
			Actions: []types.Action{
				{
					Name: "port_get",
					Conditions: []types.Condition{
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation: "port_get_rel",
							},
							RoleBinding: &types.ConditionRoleBinding{},
						},
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation:   "owner",
								ActionName: "port_get",
							},
						},
					},
				},
			},
		},
	}

	schemaOutput := `definition foo/user {
}
definition foo/client {
}
definition foo/role {
    relation subject: foo/user | foo/client
}
definition foo/tenant {
    relation parent: foo/tenant
    relation loadbalancer_create_rel: foo/role#subject
    relation loadbalancer_get_rel: foo/role#subject
    relation port_create_rel: foo/role#subject
    relation port_get_rel: foo/role#subject
    permission loadbalancer_create = loadbalancer_create_rel + parent->loadbalancer_create
    permission loadbalancer_get = loadbalancer_get_rel + parent->loadbalancer_get
    permission port_create = port_create_rel + parent->port_create
    permission port_get = port_get_rel + parent->port_get
}
definition foo/loadbalancer {
    relation owner: foo/tenant
    relation loadbalancer_get_rel: foo/role#subject
    permission loadbalancer_get = loadbalancer_get_rel + owner->loadbalancer_get
}
definition foo/port {
    relation owner: foo/tenant
    relation port_get_rel: foo/role#subject
    permission port_get = port_get_rel + owner->port_get
}
`

	testCases := []testCase{
		{
			name: "NoNamespace",
			input: testInput{
				namespace:     "",
				resourceTypes: resourceTypes,
			},
			checkFn: func(t *testing.T, res testResult) {
				assert.ErrorIs(t, res.err, ErrorNoNamespace)
				assert.Empty(t, res.success)
			},
		},
		{
			name: "SucccessNamespace",
			input: testInput{
				namespace:     "foo",
				resourceTypes: resourceTypes,
			},
			checkFn: func(t *testing.T, res testResult) {
				assert.NoError(t, res.err)
				assert.Equal(t, schemaOutput, res.success)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var result testResult

			result.success, result.err = GenerateSchema(tc.input.namespace, tc.input.resourceTypes)

			tc.checkFn(t, result)
		})
	}
}
