package iapl

import (
	"go.infratographer.com/permissions-api/internal/types"
)

const (
	// RoleOwnerRelation is the name of the relationship that connects a role to its owner.
	RoleOwnerRelation = "owner"
	// RoleOwnerMemberRoleRelation is the name of the relationship that connects a resource
	// to a role that it owns
	RoleOwnerMemberRoleRelation = "member_role"
	// AvailableRolesList is the name of the action in a resource that returns a list
	// of roles that are available for the resource
	AvailableRolesList = "avail_role"
	// RolebindingRoleRelation is the name of the relationship that connects a role binding to a role.
	RolebindingRoleRelation = "role"
	// RolebindingSubjectRelation is the name of the relationship that connects a role binding to a subject.
	RolebindingSubjectRelation = "subject"
	// RoleOwnerParentRelation is the name of the relationship that connects a role's owner to its parent.
	RoleOwnerParentRelation = "parent"
	// PermissionRelationSuffix is the suffix append to the name of the relationship
	// representing a permission in a role
	PermissionRelationSuffix = "_rel"
	// GrantRelationship is the name of the relationship that connects a role binding to a resource.
	GrantRelationship = "grant"
)

// RoleAction is the list of actions that can be performed on a role resource
type RoleAction string

const (
	// RoleActionCreate is the action name to create a role
	RoleActionCreate RoleAction = "role_create"
	// RoleActionGet is the action name to get a role
	RoleActionGet RoleAction = "role_get"
	// RoleActionList is the action name to list roles
	RoleActionList RoleAction = "role_list"
	// RoleActionUpdate is the action name to update a role
	RoleActionUpdate RoleAction = "role_update"
	// RoleActionDelete is the action name to delete a role
	RoleActionDelete RoleAction = "role_delete"
)

// RoleBindingAction is the list of actions that can be performed on a role-binding resource
type RoleBindingAction string

const (
	// RoleBindingActionCreate is the action name to create a role binding
	RoleBindingActionCreate RoleBindingAction = "iam_rolebinding_create"
	// RoleBindingActionUpdate is the action name to update a role binding
	RoleBindingActionUpdate RoleBindingAction = "iam_rolebinding_update"
	// RoleBindingActionDelete is the action name to delete a role binding
	RoleBindingActionDelete RoleBindingAction = "iam_rolebinding_delete"
	// RoleBindingActionGet is the action name to get a role binding
	RoleBindingActionGet RoleBindingAction = "iam_rolebinding_get"
	// RoleBindingActionList is the action name to list role bindings
	RoleBindingActionList RoleBindingAction = "iam_rolebinding_list"
)

// ResourceRoleBindingV2 describes the relationships that will be created
// for a resource to support role-binding V2
type ResourceRoleBindingV2 struct {
	// InheritPermissionsFrom is the list of resource types that can provide roles
	// and grants to this resource
	// Note that not all roles are available to all resources. This relationship is used to
	// determine which roles are available to a resource.
	// Before creating a role binding for a resource, one should check whether or
	// not the role is available for the resource.
	//
	// Also see the RoleOwners field in the RBAC struct
	InheritPermissionsFrom []string

	// InheritAllActions adds relationship action lookups for all actions, not just role binding actions.
	InheritAllActions bool
}

/*
RBAC represents a role-based access control policy.

For example, consider the following spicedb schema:
```zed

	definition user {}
	definition client {}

	definition group {
		relation member: user | client
	}

	definition organization {
		relation parent: organization
		relation member: user | client
		relation member_role: role
		relation grant: rolebinding

		permissions rolebinding_list: grant->rolebinding_list
		permissions rolebinding_create: grant->rolebinding_create
		permissions rolebinding_delete: grant->rolebinding_delete
	}

	definition role {
		relation owner: organization
		relation view_organization: user:* | client:*
		relation rolebinding_list_rel: user:* | client:*
		relation rolebinding_create_rel: user:* | client:*
		relation rolebinding_delete_rel: user:* | client:*
	}

	definition rolebinding {
		relation role: role
		relation subject: user | group#member
		permission view_organization = subject & role->view_organization
		permissions rolebinding_list: subject & role->rolebinding_list
		permissions rolebinding_create: subject & role->rolebinding_create
		permissions rolebinding_delete: subject & role->rolebinding_delete
	}

```
in IAPL policy terms:
- the RoleResource would be "{name: role, idprefix: someprefix}"
- the RoleBindingResource would be "{name: rolebinding, idprefix: someprefix}",
- the RoleRelationshipSubject would be `[user, client]`.
- the RoleBindingSubjects would be `[{name: user}, {name: group, subjectrelation: member}]`.
*/
type RBAC struct {
	// RoleResource is the name of the resource type that represents a role.
	RoleResource RBACResourceDefinition
	// RoleBindingResource is the name of the resource type that represents a role binding.
	RoleBindingResource RBACResourceDefinition
	// RoleSubjectTypes is a list of subject types that the relationships in a
	// role resource will contain, see the example above.
	RoleSubjectTypes []string
	// RoleOwners is the list of resource types that can own a role.
	// These resources should be (but not limited to) organizational resources
	// like tenant, organization, project, group, etc
	// When a role is owned by an entity, say a group, that means this role
	// will be available to perform role-bindings for resources that are owned
	// by this group and its subgroups.
	// The RoleOwners relationship is particularly useful to limit access to
	// custom roles.
	RoleOwners []string
	// RoleBindingSubjects is the names of the resource types that can be subjects in a role binding.
	// e.g. rolebinding_create, rolebinding_list, rolebinding_delete
	RoleBindingSubjects []types.TargetType

	roleownersset map[string]struct{}
}

// RBACResourceDefinition is a struct to define a resource type for a role
// and role-bindings
type RBACResourceDefinition struct {
	Name     string
	IDPrefix string
}

// CreateRoleBindingConditionsForAction creates the conditions that is used for role binding v2,
// for a given action name. e.g. for a doc_read action, it will create the following conditions:
// doc_read = grant->doc_read + from[0]->doc_read + ... from[n]->doc_read
func (r *RBAC) CreateRoleBindingConditionsForAction(actionName string, inheritFrom ...string) []types.Condition {
	conds := make([]types.Condition, 0, len(inheritFrom)+1)

	conds = append(conds, types.Condition{
		RelationshipAction: &types.ConditionRelationshipAction{
			Relation:   GrantRelationship,
			ActionName: actionName,
		},
		RoleBindingV2: &types.ConditionRoleBindingV2{},
	})

	for _, from := range inheritFrom {
		conds = append(conds, types.Condition{
			RelationshipAction: &types.ConditionRelationshipAction{
				Relation:   from,
				ActionName: actionName,
			},
		})
	}

	return conds
}

// CreateRoleBindingActionsForResource should be used when an RBAC V2 condition
// is created for an action, the resource that the action is belong to must
// support role binding V2. This function creates the list of actions that can be performed
// on a role binding resource.
// e.g. If action `read_doc` is created with RBAC V2 condition, then the resource,
// in this example `doc`, must also support actions like `rolebinding_create`.
func (r *RBAC) CreateRoleBindingActionsForResource(inheritFrom ...string) []types.Action {
	actionsStr := []RoleBindingAction{
		RoleBindingActionCreate,
		RoleBindingActionUpdate,
		RoleBindingActionDelete,
		RoleBindingActionGet,
		RoleBindingActionList,
	}

	actions := make([]types.Action, 0, len(actionsStr))

	for _, action := range actionsStr {
		conditions := r.CreateRoleBindingConditionsForAction(string(action), inheritFrom...)
		actions = append(actions, types.Action{Name: string(action), Conditions: conditions})
	}

	return actions
}

// RoleBindingActions returns the list of actions that can be performed on a role resource
// plus the AvailableRoleRelation action that is used to decide whether or not
// a role is available for a resource
func (r *RBAC) RoleBindingActions() []Action {
	actionsStr := []RoleBindingAction{
		RoleBindingActionCreate,
		RoleBindingActionUpdate,
		RoleBindingActionDelete,
		RoleBindingActionGet,
		RoleBindingActionList,
	}

	actions := make([]Action, 0, len(actionsStr)+1)

	for _, action := range actionsStr {
		actions = append(actions, Action{Name: string(action)})
	}

	actions = append(actions, Action{Name: AvailableRolesList})

	return actions
}

// RoleOwnersSet returns the set of role owners for easy role owner lookups
func (r *RBAC) RoleOwnersSet() map[string]struct{} {
	if r.roleownersset == nil {
		r.roleownersset = make(map[string]struct{}, len(r.RoleOwners))
		for _, owner := range r.RoleOwners {
			r.roleownersset[owner] = struct{}{}
		}
	}

	return r.roleownersset
}
