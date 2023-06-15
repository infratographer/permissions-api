package iapl

// DefaultPolicy generates the default policy for permissions-api.
func DefaultPolicy() Policy {
	policyDocument := PolicyDocument{
		ResourceTypes: []ResourceType{
			{
				Name:     "role",
				IDPrefix: "idenrol",
				Relationships: []Relationship{
					{
						Relation: "subject",
						TargetTypeNames: []string{
							"subject",
						},
					},
				},
			},
			{
				Name:     "user",
				IDPrefix: "idenusr",
			},
			{
				Name:     "client",
				IDPrefix: "idencli",
			},
			{
				Name:     "tenant",
				IDPrefix: "identen",
				Relationships: []Relationship{
					{
						Relation: "parent",
						TargetTypeNames: []string{
							"tenant",
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
						TargetTypeNames: []string{
							"resourceowner",
						},
					},
				},
			},
		},
		Unions: []Union{
			{
				Name: "subject",
				ResourceTypeNames: []string{
					"user",
					"client",
				},
			},
			{
				Name: "resourceowner",
				ResourceTypeNames: []string{
					"tenant",
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

	policy := NewPolicy(policyDocument)
	if err := policy.Validate(); err != nil {
		panic(err)
	}

	return policy
}
