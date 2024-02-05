package iapl

import (
	"fmt"
	"os"

	"go.infratographer.com/permissions-api/internal/types"

	"gopkg.in/yaml.v3"
)

// PolicyDocument represents a partial authorization policy.
type PolicyDocument struct {
	ResourceTypes  []ResourceType
	Unions         []Union
	Actions        []Action
	ActionBindings []ActionBinding
	RBAC           RBAC
}

/*
RBAC represents a role-based access control policy.
- RoleResource is the name of the resource type that represents a role.
- RoleRelationshipSubject is the name of the relationship that connects a role to a subject.
- RoleOwners is the names of the resource types that can own a role.
- RoleBindingResource is the name of the resource type that represents a role binding.
- RoleBindingSubjects is the names of the resource types that can be subjects in a role binding.

For example, consider the following RBAC policy:
```zed

	definition user {}
	definition client {}

	definition group {
		relation member: user | client
	}

	definition role {
		relation owner: organization
		relation view_organization: user:* | client:*
	}

	definition role_binding {
		relation role: role
		relation subject: user | group#member
		permission view_organization = subject & role->view_organization
	}

```

- the RoleResource would be "role"
- the RoleBindingResource would be "role_binding",
- the RoleRelationshipSubject would be `[user, client]`.
- the RoleBindingSubjects would be `[{name: user}, {name: group, subjectrelation: member}]`.
*/
type RBAC struct {
	RoleResource             string
	RoleRelationshipSubjects []string
	RoleOwners               []string

	RoleBindingResource string
	RoleBindingSubjects []types.TargetType
}

// ResourceType represents a resource type in the authorization policy.
type ResourceType struct {
	Name          string
	IDPrefix      string
	Relationships []Relationship
}

// Relationship represents a named relation between two resources.
type Relationship struct {
	Relation        string
	TargetTypeNames []string
	TargetTypes     []types.TargetType
}

// Union represents a named union of multiple concrete resource types.
type Union struct {
	Name              string
	ResourceTypeNames []string
	ResourceTypes     []types.TargetType
}

// Action represents an action that can be taken in an authorization policy.
// AdditionalPermissions can be defined to allow additional permissions to be
// created in addition to the permission with the same name as the action.
type Action struct {
	Name string
}

// ActionBinding represents a binding of an action to a resource type or union.
// RenamePermission is the name of the permission that should be used instead of
// the action name when checking for permission to perform the action, this
// allows the permission to be renamed to avoid conflicts with relationships
// with the same name.
type ActionBinding struct {
	ActionName       string
	TypeName         string
	RenamePermission string
	Conditions       []Condition
	ConditionSets    []types.ConditionSet
}

// Condition represents a necessary condition for performing an action.
type Condition struct {
	RoleBinding        *ConditionRoleBinding
	RelationshipAction *ConditionRelationshipAction
}

// ConditionRoleBinding represents a condition where a role binding is necessary to perform an action.
type ConditionRoleBinding struct{}

// ConditionRelationshipAction represents a condition where another action must be allowed on a resource
// along a relation to perform an action.
type ConditionRelationshipAction struct {
	Relation   string
	ActionName string
}

// Policy represents an authorization policy as defined by IAPL.
type Policy interface {
	Validate() error
	Schema() []types.ResourceType
	RBAC() RBAC
}

var _ Policy = &policy{}

type policy struct {
	rt map[string]ResourceType
	un map[string]Union
	ac map[string]Action
	rb map[string]map[string]struct{}
	bn []ActionBinding
	p  PolicyDocument

	permissions []string
}

// NewPolicy creates a policy from the given policy document.
func NewPolicy(p PolicyDocument) Policy {
	rt := make(map[string]ResourceType, len(p.ResourceTypes))
	for _, r := range p.ResourceTypes {
		rt[r.Name] = r
	}

	un := make(map[string]Union, len(p.Unions))
	for _, t := range p.Unions {
		un[t.Name] = t
	}

	ac := make(map[string]Action, len(p.Actions))
	for _, a := range p.Actions {
		ac[a.Name] = a
	}

	out := policy{
		rt:          rt,
		un:          un,
		ac:          ac,
		p:           p,
		permissions: []string{},
	}

	out.expandRole()
	out.expandRoleBinding()
	out.expandActionBindings()
	out.expandResourceTypes()

	return &out
}

// NewPolicyFromFile reads the provided file path and returns a new Policy.
func NewPolicyFromFile(filePath string) (Policy, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	var policy PolicyDocument

	if err := yaml.NewDecoder(file).Decode(&policy); err != nil {
		return nil, err
	}

	return NewPolicy(policy), nil
}

func (v *policy) validateUnions() error {
	for _, union := range v.p.Unions {
		if _, ok := v.rt[union.Name]; ok {
			return fmt.Errorf("%s: %w", union.Name, ErrorTypeExists)
		}

		for _, rtName := range union.ResourceTypeNames {
			if _, ok := v.rt[rtName]; !ok {
				return fmt.Errorf("%s: resourceTypeNames: %s: %w", union.Name, rtName, ErrorUnknownType)
			}
		}

		for _, rt := range union.ResourceTypes {
			if _, ok := v.rt[rt.Name]; !ok {
				return fmt.Errorf("%s: resourceTypes: %s: %w", union.Name, rt.Name, ErrorUnknownType)
			}
		}
	}

	return nil
}

func (v *policy) validateResourceTypes() error {
	findRelationship := func(rels []Relationship, name string) bool {
		for _, rel := range rels {
			if rel.Relation == name {
				return true
			}
		}

		return false
	}

	for _, resourceType := range v.p.ResourceTypes {
		for _, rel := range resourceType.Relationships {
			for _, name := range rel.TargetTypeNames {
				if _, ok := v.rt[name]; !ok {
					return fmt.Errorf("%s: relationships: %s: %w", resourceType.Name, name, ErrorUnknownType)
				}
			}

			for _, tt := range rel.TargetTypes {
				if _, ok := v.rt[tt.Name]; !ok {
					return fmt.Errorf("%s: relationships: %s: %w", resourceType.Name, tt.Name, ErrorUnknownType)
				}

				if tt.SubjectRelation != "" && !findRelationship(v.rt[tt.Name].Relationships, tt.SubjectRelation) {
					return fmt.Errorf("%s: subject-relation: %s: %w", resourceType.Name, tt.SubjectRelation, ErrorUnknownRelation)
				}
			}
		}
	}

	return nil
}

func (v *policy) validateConditionRelationshipAction(rt ResourceType, c ConditionRelationshipAction) error {
	var (
		rel   Relationship
		found bool
	)

	for _, candidate := range rt.Relationships {
		if c.Relation == candidate.Relation {
			rel = candidate
			found = true

			break
		}
	}

	if !found {
		return fmt.Errorf("%s: %w", c.Relation, ErrorUnknownRelation)
	}

	for _, tn := range rel.TargetTypeNames {
		if _, ok := v.rb[tn][c.ActionName]; !ok {
			return fmt.Errorf("%s: %s: %s: %w", c.Relation, tn, c.ActionName, ErrorUnknownAction)
		}
	}

	return nil
}

func (v *policy) validateConditions(rt ResourceType, conds []Condition) error {
	for i, cond := range conds {
		var numClauses int
		if cond.RoleBinding != nil {
			numClauses++
		}

		if cond.RelationshipAction != nil {
			numClauses++
		}

		if numClauses != 1 {
			return fmt.Errorf("%d: %w", i, ErrorInvalidCondition)
		}

		if cond.RelationshipAction != nil {
			if err := v.validateConditionRelationshipAction(rt, *cond.RelationshipAction); err != nil {
				return fmt.Errorf("%d: %w", i, err)
			}
		}
	}

	return nil
}

func (v *policy) validateActionBindings() error {
	for i, binding := range v.bn {
		if _, ok := v.ac[binding.ActionName]; !ok {
			return fmt.Errorf("%d: %s: %w", i, binding.ActionName, ErrorUnknownAction)
		}

		rt, ok := v.rt[binding.TypeName]
		if !ok {
			return fmt.Errorf("%d: %s: %w", i, binding.TypeName, ErrorUnknownType)
		}

		if err := v.validateConditions(rt, binding.Conditions); err != nil {
			return fmt.Errorf("%d: conditions: %w", i, err)
		}
	}

	return nil
}

func (v *policy) expandActionBindings() {
	for _, bn := range v.p.ActionBindings {
		if u, ok := v.un[bn.TypeName]; ok {
			for _, typeName := range u.ResourceTypeNames {
				binding := ActionBinding{
					TypeName:      typeName,
					ActionName:    bn.ActionName,
					Conditions:    bn.Conditions,
					ConditionSets: bn.ConditionSets,
				}
				v.bn = append(v.bn, binding)
			}

			for _, tt := range u.ResourceTypes {
				binding := ActionBinding{
					TypeName:      tt.Name,
					ActionName:    bn.ActionName,
					Conditions:    bn.Conditions,
					ConditionSets: bn.ConditionSets,
				}
				v.bn = append(v.bn, binding)
			}
		} else {
			v.bn = append(v.bn, bn)
		}
	}

	v.rb = make(map[string]map[string]struct{}, len(v.p.ResourceTypes))
	for _, ab := range v.bn {
		b, ok := v.rb[ab.TypeName]
		if !ok {
			b = make(map[string]struct{})
			v.rb[ab.TypeName] = b
		}

		b[ab.ActionName] = struct{}{}
	}
}

// expandRole creates a list of all permissions, and a resource containing
// a list of relationship to all permissions.
func (v *policy) expandRole() {
	// 1. create a list of all permissions
	perms := make(map[string]struct{})

	for _, action := range v.ac {
		perms[action.Name] = struct{}{}
	}

	for perm := range perms {
		v.permissions = append(v.permissions, perm)
	}

	// 2. create a relationship for role owners
	roleOwners := Relationship{
		Relation:    "owner",
		TargetTypes: make([]types.TargetType, len(v.p.RBAC.RoleOwners)),
	}

	for i, owner := range v.p.RBAC.RoleOwners {
		roleOwners.TargetTypes[i] = types.TargetType{Name: owner}
	}

	// 3. create a list of relationships for all permissions
	permsRel := make([]Relationship, len(perms))

	for i, perm := range v.permissions {
		targettypes := make([]types.TargetType, len(v.p.RBAC.RoleRelationshipSubjects))

		for j, subject := range v.p.RBAC.RoleRelationshipSubjects {
			targettypes[j] = types.TargetType{Name: subject, SubjectIdentifier: "*"}
		}

		permsRel[i] = Relationship{
			Relation:    perm + "_rel",
			TargetTypes: targettypes,
		}
	}

	// 4. create a role containing all the relationships shown above
	var role ResourceType

	permsRel = append(permsRel, roleOwners)

	if _, ok := v.rt[v.p.RBAC.RoleResource]; ok {
		role = v.rt[v.p.RBAC.RoleResource]
		role.Relationships = permsRel
	} else {
		role = ResourceType{
			Name:          v.p.RBAC.RoleResource,
			Relationships: permsRel,
		}
	}

	v.rt[role.Name] = role
}

func (v *policy) expandRoleBinding() {
	// 1. create relationship to role
	role := Relationship{
		Relation: "role",
		TargetTypes: []types.TargetType{
			{Name: v.p.RBAC.RoleResource},
		},
	}

	// 2. create relationship to subjects
	subjects := Relationship{
		Relation:    "subject",
		TargetTypes: v.p.RBAC.RoleBindingSubjects,
	}

	// 3. create a list of action-bindings representing permissions for all the
	// actions
	actionbindings := make([]ActionBinding, len(v.ac))
	i := 0

	for actionName := range v.ac {
		ab := ActionBinding{
			ActionName: actionName,
			TypeName:   v.p.RBAC.RoleBindingResource,
			ConditionSets: []types.ConditionSet{
				{
					Conditions: []types.Condition{
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation:   v.p.RBAC.RoleResource,
								ActionName: actionName + "_rel",
							},
						},
					},
				},
				{
					Conditions: []types.Condition{
						{RelationshipAction: &types.ConditionRelationshipAction{Relation: "subject"}},
					},
				},
			},
		}

		actionbindings[i] = ab
		i++
	}

	v.bn = append(v.bn, actionbindings...)

	// 4. create role-binding resource type
	var rolebinding ResourceType

	if _, ok := v.rt[v.p.RBAC.RoleBindingResource]; ok {
		rolebinding = v.rt[v.p.RBAC.RoleBindingResource]
		rolebinding.Relationships = []Relationship{role, subjects}
	} else {
		rolebinding = ResourceType{
			Name:          v.p.RBAC.RoleBindingResource,
			Relationships: []Relationship{role, subjects},
		}
	}

	v.rt[v.p.RBAC.RoleBindingResource] = rolebinding
}

func (v *policy) expandResourceTypes() {
	for name, resourceType := range v.rt {
		for i, rel := range resourceType.Relationships {
			var typeNames []string

			targettypes := rel.TargetTypes

			for _, typeName := range rel.TargetTypeNames {
				if u, ok := v.un[typeName]; ok {
					if len(u.ResourceTypes) > 0 {
						targettypes = append(targettypes, u.ResourceTypes...)
					} else {
						typeNames = append(typeNames, u.ResourceTypeNames...)
					}
				} else {
					typeNames = append(typeNames, typeName)
				}
			}

			for _, tn := range typeNames {
				targettypes = append(targettypes, types.TargetType{Name: tn})
			}

			resourceType.Relationships[i].TargetTypeNames = typeNames
			resourceType.Relationships[i].TargetTypes = targettypes
		}

		v.rt[name] = resourceType
	}
}

func (v *policy) Validate() error {
	if err := v.validateUnions(); err != nil {
		return fmt.Errorf("unions: %w", err)
	}

	if err := v.validateResourceTypes(); err != nil {
		return fmt.Errorf("resourceTypes: %w", err)
	}

	if err := v.validateActionBindings(); err != nil {
		return fmt.Errorf("actionBindings: %w", err)
	}

	return nil
}

func (v *policy) Schema() []types.ResourceType {
	typeMap := map[string]*types.ResourceType{}

	for n, rt := range v.rt {
		out := types.ResourceType{
			Name:     rt.Name,
			IDPrefix: rt.IDPrefix,
		}

		for _, rel := range rt.Relationships {
			outRel := types.ResourceTypeRelationship{
				Relation: rel.Relation,
				Types:    rel.TargetTypes,
			}

			out.Relationships = append(out.Relationships, outRel)
		}

		typeMap[n] = &out
	}

	for _, b := range v.bn {
		actionName := b.ActionName

		if b.RenamePermission != "" {
			actionName = b.RenamePermission
		}

		action := types.Action{
			Name: actionName,
		}

		for _, c := range b.Conditions {
			condition := types.Condition{
				RoleBinding:        (*types.ConditionRoleBinding)(c.RoleBinding),
				RelationshipAction: (*types.ConditionRelationshipAction)(c.RelationshipAction),
			}

			action.Conditions = append(action.Conditions, condition)
		}

		action.ConditionSets = b.ConditionSets

		typeMap[b.TypeName].Actions = append(typeMap[b.TypeName].Actions, action)
	}

	out := make([]types.ResourceType, len(v.p.ResourceTypes))
	for i, rt := range v.p.ResourceTypes {
		out[i] = *typeMap[rt.Name]
	}

	return out
}

// RBAC returns the RBAC configurations
func (v *policy) RBAC() RBAC {
	return v.p.RBAC
}
