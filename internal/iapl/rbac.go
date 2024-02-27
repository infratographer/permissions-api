package iapl

import (
	"go.infratographer.com/permissions-api/internal/types"
)

/*
RBAC represents a role-based access control policy.
  - RoleResource is the name of the resource type that represents a role.
  - RoleRelationshipSubject is the name of the relationship that connects a role to a subject.
  - RoleOwners is the names of the resource types that can own a role.
  - RoleBindingResource is the name of the resource type that represents a role binding.
  - RoleBindingSubjects is the names of the resource types that can be subjects in a role binding.
  - RolebindingPermissionsPrefix generates the permissions sets to manage role bindings,
  - GrantRelationship is the name of the relationship that connects a role binding to a resource.
    e.g. rolebinding_create, rolebinding_list, rolebinding_delete

For example, consider the following spicedb schema:
```zed

	definition user {}
	definition client {}

	definition group {
		relation member: user | client
	}

	definition organization {
		relation parent: organization
		relation member: user | client | organization#member
		relation member_role: role | organization#member_role
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

	definition role_binding {
		relation role: role
		relation subject: user | group#member
		permission view_organization = subject & role->view_organization
		permissions rolebinding_list: subject & role->rolebinding_list
		permissions rolebinding_create: subject & role->rolebinding_create
		permissions rolebinding_delete: subject & role->rolebinding_delete
	}

```
in IAPL policy terms:
- the RoleResource would be "role"
- the RoleBindingResource would be "role_binding",
- the RoleRelationshipSubject would be `[user, client]`.
- the RoleBindingSubjects would be `[{name: user}, {name: group, subjectrelation: member}]`.
- the RolebindingPermissionsPrefix would be "rolebinding"
- the GrantRelationship would be "grant"
*/
type RBAC struct {
	RoleResource                 string
	RoleRelationshipSubjects     []string
	RoleOwners                   []string
	RoleBindingResource          string
	RoleBindingSubjects          []types.TargetType
	RoleBindingPermissionsPrefix string
	GrantRelationship            string
}

// UnmarshalYAML is a custom YAML unmarshaller for the RBAC policy, with
// default values set for the fields.
func (r *RBAC) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rbac RBAC

	rbacYAML := rbac(DefaultRBAC())

	if err := unmarshal(&rbacYAML); err != nil {
		return err
	}

	*r = RBAC(rbacYAML)

	return nil
}

// DefaultRBAC returns the default values for the RBAC policy.
// the default values are:
// rbac:
//
//	roleresource: rolev2
//	rolerelationshipsubjects:
//	  - user
//	  - client
//	roleowners:
//	  - tenant
//	rolebindingresource: role_binding
//	rolebindingsubjects:
//	  - name: user
//	  - name: client
//	  - name: group
//	    subjectrelation: member
//	rolebindingpermissionsprefix: rolebinding
func DefaultRBAC() RBAC {
	return RBAC{
		RoleResource:                 "rolev2",
		RoleRelationshipSubjects:     []string{"user", "client"},
		RoleOwners:                   []string{"tenant"},
		RoleBindingResource:          "role_binding",
		RoleBindingSubjects:          []types.TargetType{{Name: "user"}, {Name: "client"}, {Name: "group", SubjectRelation: "member"}},
		RoleBindingPermissionsPrefix: "rolebinding",
		GrantRelationship:            "grant",
	}
}
