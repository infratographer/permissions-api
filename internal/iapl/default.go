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
						TargetTypeNames: []string{
							"subject",
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

// DefaultPolicyDocumentV2 returns the default policy document that supports
// RBAC V2
func DefaultPolicyDocumentV2() PolicyDocument {
	rbv2WithInheritFromOwner := Condition{
		RoleBindingV2: &ConditionRoleBindingV2{
			InheritGrantsFrom: []string{"owner"},
		},
	}

	rbv2WithInheritFromParent := Condition{
		RoleBindingV2: &ConditionRoleBindingV2{
			InheritGrantsFrom: []string{"parent"},
		},
	}

	return PolicyDocument{
		RBAC: &RBAC{
			RoleResource:        "rolev2",
			RoleSubjectTypes:    []string{"user", "client"},
			RoleOwners:          []string{"tenant"},
			RoleBindingResource: "role_binding",
			RoleBindingSubjects: []types.TargetType{
				{Name: "user"},
				{Name: "client"},
				{Name: "group", SubjectRelation: "member"},
			},
		},
		Unions: []Union{
			{
				Name: "group_member",
				ResourceTypes: []types.TargetType{
					{Name: "user"},
					{Name: "client"},
					{Name: "group", SubjectRelation: "member"},
				},
			},
			{
				Name: "tenant_member",
				ResourceTypes: []types.TargetType{
					{Name: "user"},
					{Name: "client"},
					{Name: "group", SubjectRelation: "member"},
					{Name: "tenant", SubjectRelation: "member"},
				},
			},
			{
				Name: "resourceowner",
				ResourceTypes: []types.TargetType{
					{Name: "tenant"},
				},
			},
			{
				Name: "resourceowner_relationship",
				ResourceTypes: []types.TargetType{
					{Name: "tenant"}, {Name: "group", SubjectRelation: "parent"},
				},
			},
			{
				Name:              "subject",
				ResourceTypeNames: []string{"user", "client"},
			},
			{
				Name: "group_parent",
				ResourceTypes: []types.TargetType{
					{Name: "group"},
					{Name: "group", SubjectRelation: "parent"},
					{Name: "tenant"},
					{Name: "tenant", SubjectRelation: "parent"},
				},
			},
			{
				Name: "tenant_parent",
				ResourceTypes: []types.TargetType{
					{Name: "tenant"}, {Name: "tenant", SubjectRelation: "parent"},
				},
			},
		},
		ResourceTypes: []ResourceType{
			{Name: "rolev2", IDPrefix: "permrv2"},
			{Name: "role_binding", IDPrefix: "permrbn"},
			{Name: "user", IDPrefix: "idntusr"},
			{Name: "client", IDPrefix: "idntclt"},
			{
				Name:     "group",
				IDPrefix: "idntgrp",
				RoleBindingV2: &ResourceRoleBindingV2{
					AvailableRolesFrom: []string{"parent"},
				},
				Relationships: []Relationship{
					{Relation: "parent", TargetTypeNames: []string{"group_parent"}},
					{Relation: "member", TargetTypeNames: []string{"group_member"}},
					{Relation: "grant", TargetTypeNames: []string{"role_binding"}},
				},
			},
			{
				Name:     "tenant",
				IDPrefix: "tnntten",
				RoleBindingV2: &ResourceRoleBindingV2{
					AvailableRolesFrom: []string{"parent"},
				},
				Relationships: []Relationship{
					{Relation: "parent", TargetTypeNames: []string{"tenant_parent"}},
					{Relation: "member", TargetTypeNames: []string{"tenant_member"}},
					{Relation: "grant", TargetTypeNames: []string{"role_binding"}},
				},
			},
			{
				Name:     "loadbalancer",
				IDPrefix: "loadbal",
				RoleBindingV2: &ResourceRoleBindingV2{
					AvailableRolesFrom: []string{"owner"},
				},
				Relationships: []Relationship{
					{Relation: "owner", TargetTypeNames: []string{"resourceowner_relationship"}},
					{Relation: "grant", TargetTypeNames: []string{"role_binding"}},
				},
			},
		},
		Actions: []Action{
			{Name: "role_create"},
			{Name: "role_get"},
			{Name: "role_list"},
			{Name: "role_update"},
			{Name: "role_delete"},
			{Name: "loadbalancer_create"},
			{Name: "loadbalancer_get"},
			{Name: "loadbalancer_list"},
			{Name: "loadbalancer_update"},
			{Name: "loadbalancer_delete"},
		},
		ActionBindings: []ActionBinding{
			{
				ActionName: "role_get",
				TypeName:   "rolev2",
				Conditions: []Condition{
					{
						RelationshipAction: &ConditionRelationshipAction{
							Relation:   "owner",
							ActionName: "role_get",
						},
					},
				},
			},
			{
				ActionName: "role_update",
				TypeName:   "rolev2",
				Conditions: []Condition{
					{
						RelationshipAction: &ConditionRelationshipAction{
							Relation:   "owner",
							ActionName: "role_update",
						},
					},
				},
			},
			{
				ActionName: "role_delete",
				TypeName:   "rolev2",
				Conditions: []Condition{
					{
						RelationshipAction: &ConditionRelationshipAction{
							Relation:   "owner",
							ActionName: "role_delete",
						},
					},
				},
			},
			{
				ActionName: "role_create",
				TypeName:   "resourceowner",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_create",
				TypeName:   "group",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_get",
				TypeName:   "resourceowner",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_get",
				TypeName:   "group",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_list",
				TypeName:   "resourceowner",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_list",
				TypeName:   "group",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_update",
				TypeName:   "resourceowner",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_update",
				TypeName:   "group",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_delete",
				TypeName:   "resourceowner",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_delete",
				TypeName:   "group",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_get",
				TypeName:   "loadbalancer",
				Conditions: []Condition{rbv2WithInheritFromOwner},
			},
			{
				ActionName: "loadbalancer_update",
				TypeName:   "loadbalancer",
				Conditions: []Condition{rbv2WithInheritFromOwner},
			},
			{
				ActionName: "loadbalancer_delete",
				TypeName:   "loadbalancer",
				Conditions: []Condition{rbv2WithInheritFromOwner},
			},
			{
				ActionName: "loadbalancer_create",
				TypeName:   "resourceowner",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_create",
				TypeName:   "group",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_get",
				TypeName:   "resourceowner",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_get",
				TypeName:   "group",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_list",
				TypeName:   "resourceowner",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_list",
				TypeName:   "group",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_update",
				TypeName:   "resourceowner",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_update",
				TypeName:   "group",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_delete",
				TypeName:   "resourceowner",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_delete",
				TypeName:   "group",
				Conditions: []Condition{rbv2WithInheritFromParent},
			},
		},
	}
}

// DefaultPolicyV2 generates the default policy for permissions-api that supports
// RBAC V2
func DefaultPolicyV2() Policy {
	policyDocument := DefaultPolicyDocumentV2()

	policy := NewPolicy(policyDocument)
	if err := policy.Validate(); err != nil {
		panic(err)
	}

	return policy
}
