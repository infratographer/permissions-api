package query

import (
	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/types"
)

// DefaultPolicyDocumentV2 returns the default policy document that supports
// RBAC V2
func DefaultPolicyDocumentV2() iapl.PolicyDocument {
	rbv2WithInheritFromOwner := iapl.Condition{
		RoleBindingV2: &iapl.ConditionRoleBindingV2{},
	}

	rbv2WithInheritFromParent := iapl.Condition{
		RoleBindingV2: &iapl.ConditionRoleBindingV2{},
	}

	return iapl.PolicyDocument{
		RBAC: &iapl.RBAC{
			RoleResource: iapl.RBACResourceDefinition{
				Name:     "rolev2",
				IDPrefix: "permrv2",
			},
			RoleBindingResource: iapl.RBACResourceDefinition{
				Name:     "rolebinding",
				IDPrefix: "permrbn",
			},
			RoleSubjectTypes: []string{"user", "client"},
			RoleOwners:       []string{"tenant"},
			RoleBindingSubjects: []types.TargetType{
				{Name: "user"},
				{Name: "client"},
				{Name: "group", SubjectRelation: "member"},
			},
		},
		Unions: []iapl.Union{
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
				Name: "subject",
				ResourceTypes: []types.TargetType{
					{Name: "user"}, {Name: "client"},
				},
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
		ResourceTypes: []iapl.ResourceType{
			{Name: "rolev2", IDPrefix: "permrv2"},
			{Name: "rolebinding", IDPrefix: "permrbn"},
			{Name: "user", IDPrefix: "idntusr"},
			{Name: "client", IDPrefix: "idntclt"},
			{
				Name:     "group",
				IDPrefix: "idntgrp",
				RoleBindingV2: &iapl.ResourceRoleBindingV2{
					InheritPermissionsFrom: []string{"parent"},
				},
				Relationships: []iapl.Relationship{
					{Relation: "parent", TargetTypes: []types.TargetType{{Name: "group_parent"}}},
					{Relation: "member", TargetTypes: []types.TargetType{{Name: "group_member"}}},
					{Relation: "grant", TargetTypes: []types.TargetType{{Name: "rolebinding"}}},
				},
			},
			{
				Name:     "tenant",
				IDPrefix: "tnntten",
				RoleBindingV2: &iapl.ResourceRoleBindingV2{
					InheritPermissionsFrom: []string{"parent"},
				},
				Relationships: []iapl.Relationship{
					{Relation: "parent", TargetTypes: []types.TargetType{{Name: "tenant_parent"}}},
					{Relation: "member", TargetTypes: []types.TargetType{{Name: "tenant_member"}}},
					{Relation: "grant", TargetTypes: []types.TargetType{{Name: "rolebinding"}}},
				},
			},
			{
				Name:     "loadbalancer",
				IDPrefix: "loadbal",
				RoleBindingV2: &iapl.ResourceRoleBindingV2{
					InheritPermissionsFrom: []string{"owner"},
				},
				Relationships: []iapl.Relationship{
					{Relation: "owner", TargetTypes: []types.TargetType{{Name: "resourceowner_relationship"}}},
					{Relation: "grant", TargetTypes: []types.TargetType{{Name: "rolebinding"}}},
				},
			},
		},
		Actions: []iapl.Action{
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
		ActionBindings: []iapl.ActionBinding{
			{
				ActionName: "role_get",
				TypeName:   "rolev2",
				Conditions: []iapl.Condition{
					{
						RelationshipAction: &iapl.ConditionRelationshipAction{
							Relation:   "owner",
							ActionName: "role_get",
						},
					},
				},
			},
			{
				ActionName: "role_update",
				TypeName:   "rolev2",
				Conditions: []iapl.Condition{
					{
						RelationshipAction: &iapl.ConditionRelationshipAction{
							Relation:   "owner",
							ActionName: "role_update",
						},
					},
				},
			},
			{
				ActionName: "role_delete",
				TypeName:   "rolev2",
				Conditions: []iapl.Condition{
					{
						RelationshipAction: &iapl.ConditionRelationshipAction{
							Relation:   "owner",
							ActionName: "role_delete",
						},
					},
				},
			},
			{
				ActionName: "role_create",
				TypeName:   "resourceowner",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_create",
				TypeName:   "group",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_get",
				TypeName:   "resourceowner",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_get",
				TypeName:   "group",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_list",
				TypeName:   "resourceowner",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_list",
				TypeName:   "group",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_update",
				TypeName:   "resourceowner",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_update",
				TypeName:   "group",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_delete",
				TypeName:   "resourceowner",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "role_delete",
				TypeName:   "group",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_get",
				TypeName:   "loadbalancer",
				Conditions: []iapl.Condition{rbv2WithInheritFromOwner},
			},
			{
				ActionName: "loadbalancer_update",
				TypeName:   "loadbalancer",
				Conditions: []iapl.Condition{rbv2WithInheritFromOwner},
			},
			{
				ActionName: "loadbalancer_delete",
				TypeName:   "loadbalancer",
				Conditions: []iapl.Condition{rbv2WithInheritFromOwner},
			},
			{
				ActionName: "loadbalancer_create",
				TypeName:   "resourceowner",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_create",
				TypeName:   "group",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_get",
				TypeName:   "resourceowner",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_get",
				TypeName:   "group",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_list",
				TypeName:   "resourceowner",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_list",
				TypeName:   "group",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_update",
				TypeName:   "resourceowner",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_update",
				TypeName:   "group",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_delete",
				TypeName:   "resourceowner",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
			{
				ActionName: "loadbalancer_delete",
				TypeName:   "group",
				Conditions: []iapl.Condition{rbv2WithInheritFromParent},
			},
		},
	}
}

// DefaultPolicyV2 generates the default policy for permissions-api that supports
// RBAC V2
func DefaultPolicyV2() iapl.Policy {
	policyDocument := DefaultPolicyDocumentV2()

	policy := iapl.NewPolicy(policyDocument)
	if err := policy.Validate(); err != nil {
		panic(err)
	}

	return policy
}
