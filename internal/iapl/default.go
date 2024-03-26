package iapl

import "go.infratographer.com/permissions-api/internal/types"

// DefaultPolicyDocument returns the default policy document for permissions-api.
func DefaultPolicyDocument() PolicyDocument {
	return PolicyDocument{
		ResourceTypes: []ResourceType{
			{
				Name:     "role",
				IDPrefix: "permrol",
				Relationships: []Relationship{
					{
						Relation: "subject",
						TargetTypes: []types.TargetType{
							{Name: "subject"},
						},
					},
				},
			},
			{
				Name:     "user",
				IDPrefix: "idntusr",
			},
			{
				Name:     "client",
				IDPrefix: "idntcli",
			},
			{
				Name:     "tenant",
				IDPrefix: "tnntten",
				Relationships: []Relationship{
					{
						Relation: "parent",
						TargetTypes: []types.TargetType{
							{Name: "tenant"},
						},
					},
				},
			},
			{
				Name:     "loadbalancer",
				IDPrefix: "loadbal",
				Relationships: []Relationship{
					{
						Relation: "owner",
						TargetTypes: []types.TargetType{
							{Name: "resourceowner"},
						},
					},
				},
			},
		},
		Unions: []Union{
			{
				Name: "subject",
				ResourceTypes: []types.TargetType{
					{Name: "user"},
					{Name: "client"},
				},
			},
			{
				Name: "resourceowner",
				ResourceTypes: []types.TargetType{
					{Name: "tenant"},
				},
			},
		},
		Actions: []Action{
			{
				Name: "loadbalancer_create",
			},
			{
				Name: "loadbalancer_get",
			},
			{
				Name: "loadbalancer_list",
			},
			{
				Name: "loadbalancer_update",
			},
			{
				Name: "loadbalancer_delete",
			},
		},
		ActionBindings: []ActionBinding{
			{
				ActionName: "loadbalancer_create",
				TypeName:   "resourceowner",
				Conditions: []Condition{
					{
						RoleBinding: &ConditionRoleBinding{},
					},
					{
						RelationshipAction: &ConditionRelationshipAction{
							Relation:   "parent",
							ActionName: "loadbalancer_create",
						},
					},
				},
			},
			{
				ActionName: "loadbalancer_get",
				TypeName:   "resourceowner",
				Conditions: []Condition{
					{
						RoleBinding: &ConditionRoleBinding{},
					},
					{
						RelationshipAction: &ConditionRelationshipAction{
							Relation:   "parent",
							ActionName: "loadbalancer_get",
						},
					},
				},
			},
			{
				ActionName: "loadbalancer_update",
				TypeName:   "resourceowner",
				Conditions: []Condition{
					{
						RoleBinding: &ConditionRoleBinding{},
					},
					{
						RelationshipAction: &ConditionRelationshipAction{
							Relation:   "parent",
							ActionName: "loadbalancer_update",
						},
					},
				},
			},
			{
				ActionName: "loadbalancer_list",
				TypeName:   "resourceowner",
				Conditions: []Condition{
					{
						RoleBinding: &ConditionRoleBinding{},
					},
					{
						RelationshipAction: &ConditionRelationshipAction{
							Relation:   "parent",
							ActionName: "loadbalancer_list",
						},
					},
				},
			},
			{
				ActionName: "loadbalancer_delete",
				TypeName:   "resourceowner",
				Conditions: []Condition{
					{
						RoleBinding: &ConditionRoleBinding{},
					},
					{
						RelationshipAction: &ConditionRelationshipAction{
							Relation:   "parent",
							ActionName: "loadbalancer_delete",
						},
					},
				},
			},
			{
				ActionName: "loadbalancer_get",
				TypeName:   "loadbalancer",
				Conditions: []Condition{
					{
						RoleBinding: &ConditionRoleBinding{},
					},
					{
						RelationshipAction: &ConditionRelationshipAction{
							Relation:   "owner",
							ActionName: "loadbalancer_get",
						},
					},
				},
			},
			{
				ActionName: "loadbalancer_update",
				TypeName:   "loadbalancer",
				Conditions: []Condition{
					{
						RoleBinding: &ConditionRoleBinding{},
					},
					{
						RelationshipAction: &ConditionRelationshipAction{
							Relation:   "owner",
							ActionName: "loadbalancer_update",
						},
					},
				},
			},
			{
				ActionName: "loadbalancer_delete",
				TypeName:   "loadbalancer",
				Conditions: []Condition{
					{
						RoleBinding: &ConditionRoleBinding{},
					},
					{
						RelationshipAction: &ConditionRelationshipAction{
							Relation:   "owner",
							ActionName: "loadbalancer_delete",
						},
					},
				},
			},
		},
	}
}

// DefaultPolicy generates the default policy for permissions-api.
func DefaultPolicy() Policy {
	policyDocument := DefaultPolicyDocument()

	policy := NewPolicy(policyDocument)
	if err := policy.Validate(); err != nil {
		panic(err)
	}

	return policy
}
