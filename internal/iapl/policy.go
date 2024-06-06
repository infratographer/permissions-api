package iapl

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.infratographer.com/permissions-api/internal/types"

	"go.infratographer.com/x/gidx"
	"gopkg.in/yaml.v3"
)

// PolicyDocument represents a partial authorization policy.
type PolicyDocument struct {
	ResourceTypes  []ResourceType
	Unions         []Union
	Actions        []Action
	ActionBindings []ActionBinding
	RBAC           *RBAC
}

// ResourceType represents a resource type in the authorization policy.
type ResourceType struct {
	Name          string
	IDPrefix      string
	RoleBindingV2 *ResourceRoleBindingV2
	Relationships []Relationship
}

// Relationship represents a named relation between two resources.
type Relationship struct {
	Relation    string
	TargetTypes []types.TargetType
}

// Union represents a named union of multiple concrete resource types.
type Union struct {
	Name          string
	ResourceTypes []types.TargetType
}

// Action represents an action that can be taken in an authorization policy.
type Action struct {
	Name string
}

// ActionBinding represents a binding of an action to a resource type or union.
type ActionBinding struct {
	ActionName    string
	TypeName      string
	Conditions    []Condition
	ConditionSets []types.ConditionSet
}

// Condition represents a necessary condition for performing an action.
type Condition struct {
	RoleBinding        *ConditionRoleBinding
	RoleBindingV2      *ConditionRoleBindingV2
	RelationshipAction *ConditionRelationshipAction
}

// ConditionRoleBinding represents a condition where a role binding is necessary to perform an action.
type ConditionRoleBinding struct{}

// ConditionRoleBindingV2 represents a condition where a role binding is necessary to perform an action.
// Using this condition type in the policy will instruct the policy engine to
// create all the necessary relationships in the schema to support RBAC V2
type ConditionRoleBindingV2 struct{}

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
	RBAC() *RBAC
}

var _ Policy = &policy{}

type policy struct {
	rt map[string]ResourceType
	un map[string]Union
	ac map[string]Action
	rb map[string]map[string]struct{}
	bn []ActionBinding
	p  PolicyDocument
}

// NewPolicy creates a policy from the given policy document.
func NewPolicy(p PolicyDocument) Policy {
	rt := make(map[string]ResourceType)
	for _, r := range p.ResourceTypes {
		rt[r.Name] = r
	}

	un := make(map[string]Union, len(p.Unions))
	for _, t := range p.Unions {
		un[t.Name] = t
	}

	ac := map[string]Action{}

	for _, a := range p.Actions {
		ac[a.Name] = a
	}

	out := policy{
		rt: rt,
		un: un,
		ac: ac,
		p:  p,
	}

	if p.RBAC != nil {
		for _, rba := range p.RBAC.RoleBindingActions() {
			out.ac[rba.Name] = rba
		}

		out.createV2RoleResourceType()
		out.createRoleBindingResourceType()
		out.expandRBACV2Relationships()
	}

	out.expandActionBindings()
	out.expandResourceTypes()

	return &out
}

// MergeWithPolicyDocument merges this document with another, returning the new PolicyDocument.
func (p PolicyDocument) MergeWithPolicyDocument(other PolicyDocument) PolicyDocument {
	p.ResourceTypes = append(p.ResourceTypes, other.ResourceTypes...)

	p.Unions = append(p.Unions, other.Unions...)

	p.Actions = append(p.Actions, other.Actions...)

	p.ActionBindings = append(p.ActionBindings, other.ActionBindings...)

	if other.RBAC != nil {
		p.RBAC = other.RBAC
	}

	return p
}

func loadPolicyDocumentFromFile(filePath string) (PolicyDocument, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return PolicyDocument{}, fmt.Errorf("%s: %w", filePath, err)
	}

	defer file.Close()

	var (
		finalPolicyDocument = PolicyDocument{}
		decoder             = yaml.NewDecoder(file)
		documentIndex       int
	)

	for {
		var policyDocument PolicyDocument

		if err = decoder.Decode(&policyDocument); err != nil {
			if !errors.Is(err, io.EOF) {
				return PolicyDocument{}, fmt.Errorf("%s document %d: %w", filePath, documentIndex, err)
			}

			break
		}

		if finalPolicyDocument.RBAC != nil && policyDocument.RBAC != nil {
			return PolicyDocument{}, fmt.Errorf("%s document %d: %w", filePath, documentIndex, ErrorDuplicateRBACDefinition)
		}

		finalPolicyDocument = finalPolicyDocument.MergeWithPolicyDocument(policyDocument)

		documentIndex++
	}

	return finalPolicyDocument, nil
}

// LoadPolicyDocumentFromFiles loads all policy documents in the order provided and returns a merged PolicyDocument.
func LoadPolicyDocumentFromFiles(filePaths ...string) (PolicyDocument, error) {
	var policyDocument PolicyDocument

	for _, filePath := range filePaths {
		filePolicyDocument, err := loadPolicyDocumentFromFile(filePath)
		if err != nil {
			return PolicyDocument{}, err
		}

		policyDocument = policyDocument.MergeWithPolicyDocument(filePolicyDocument)
	}

	return policyDocument, nil
}

// LoadPolicyDocumentFromDirectory reads the provided directory path, reads all files in the
// directory, merges them, and returns a new merged PolicyDocument. Directories beginning with "."
// are skipped.
func LoadPolicyDocumentFromDirectory(directoryPath string) (PolicyDocument, error) {
	var filePaths []string

	err := filepath.WalkDir(directoryPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories beginning with "." (i.e., hidden directories)
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}

		ext := filepath.Ext(entry.Name())

		if strings.EqualFold(ext, ".yml") || strings.EqualFold(ext, ".yaml") {
			filePaths = append(filePaths, path)
		}

		return nil
	})
	if err != nil {
		return PolicyDocument{}, err
	}

	return LoadPolicyDocumentFromFiles(filePaths...)
}

// NewPolicyFromFile reads the provided file path and returns a new Policy.
func NewPolicyFromFile(filePath string) (Policy, error) {
	policyDocument, err := LoadPolicyDocumentFromFiles(filePath)
	if err != nil {
		return nil, err
	}

	return NewPolicy(policyDocument), nil
}

// NewPolicyFromFiles reads the provided file paths, merges them, and returns a new Policy.
func NewPolicyFromFiles(filePaths []string) (Policy, error) {
	policyDocument, err := LoadPolicyDocumentFromFiles(filePaths...)
	if err != nil {
		return nil, err
	}

	return NewPolicy(policyDocument), nil
}

// NewPolicyFromDirectory reads the provided directory path, reads all files in the directory, merges them, and returns a new Policy.
func NewPolicyFromDirectory(directoryPath string) (Policy, error) {
	policyDocument, err := LoadPolicyDocumentFromDirectory(directoryPath)
	if err != nil {
		return nil, err
	}

	return NewPolicy(policyDocument), nil
}

func (v *policy) validateUnions() error {
	for _, union := range v.p.Unions {
		if _, ok := v.rt[union.Name]; ok {
			return fmt.Errorf("%s: %w", union.Name, ErrorTypeExists)
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
	for _, resourceType := range v.rt {
		if _, err := gidx.NewID(resourceType.IDPrefix); err != nil {
			return fmt.Errorf("%w: %s", err, resourceType.Name)
		}

		for _, rel := range resourceType.Relationships {
			for _, tt := range rel.TargetTypes {
				if _, ok := v.rt[tt.Name]; !ok {
					return fmt.Errorf("%s: relationships: %s: %w", resourceType.Name, tt.Name, ErrorUnknownType)
				}

				if tt.SubjectRelation != "" && !v.findRelationship(v.rt[tt.Name].Relationships, tt.SubjectRelation) && !v.findActionBinding(tt.SubjectRelation, tt.Name) {
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

	// if there's a relationship action defined with only the relation,
	// e.g.,
	// ```yaml
	// actionname: someaction
	// typename: someresource
	// conditions:
	//   - relationshipaction:
	//       relation: some_relation
	// ```
	// the above logics ensure that `some_relation` exists, thus can safely
	// return without error
	if c.ActionName == "" {
		return nil
	}

	for _, tt := range rel.TargetTypes {
		if _, ok := v.rb[tt.Name][c.ActionName]; !ok {
			return fmt.Errorf("%s: %s: %s: %w", c.Relation, tt.Name, c.ActionName, ErrorUnknownAction)
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

		if cond.RoleBindingV2 != nil {
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
	type bindingMapKey struct {
		actionName string
		typeName   string
	}

	bindingMap := make(map[bindingMapKey]struct{}, len(v.p.ActionBindings))

	for i, binding := range v.bn {
		key := bindingMapKey{
			actionName: binding.ActionName,
			typeName:   binding.TypeName,
		}

		if _, ok := bindingMap[key]; ok {
			return fmt.Errorf("%d (%s:%s): %w", i, binding.TypeName, binding.ActionName, ErrorActionBindingExists)
		}

		bindingMap[key] = struct{}{}

		if _, ok := v.ac[binding.ActionName]; !ok {
			return fmt.Errorf("%d (%s:%s): %s: %w", i, binding.TypeName, binding.ActionName, binding.ActionName, ErrorUnknownAction)
		}

		rt, ok := v.rt[binding.TypeName]
		if !ok {
			return fmt.Errorf("%d (%s:%s): %s: %w", i, binding.TypeName, binding.ActionName, binding.TypeName, ErrorUnknownType)
		}

		if err := v.validateConditions(rt, binding.Conditions); err != nil {
			return fmt.Errorf("%d (%s:%s): conditions: %w", i, binding.TypeName, binding.ActionName, err)
		}
	}

	return nil
}

// validateRoles validates V2 role resource types to ensure that:
//   - role resource type has a valid owner relationship
func (v *policy) validateRoles() error {
	if v.p.RBAC == nil {
		return nil
	}

	for _, roleOwnerName := range v.p.RBAC.RoleOwners {
		_, ok := v.rt[roleOwnerName]

		// check if role owner exists
		if !ok {
			return fmt.Errorf("%w: role owner %s does not exist", ErrorUnknownType, roleOwnerName)
		}
	}

	return nil
}

func (v *policy) expandActionBindings() {
	for _, bn := range v.p.ActionBindings {
		if u, ok := v.un[bn.TypeName]; ok {
			for _, resourceType := range u.ResourceTypes {
				binding := ActionBinding{
					TypeName:      resourceType.Name,
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

// createV2RoleResourceType creates a v2 role resource type contains a list of relationships
// representing all the actions, as well as relationships and permissions for
// the management of the roles themselves.
// This is equivalent to including a resource that looks like:
//
//	name: rolev2
//	idprefix: permrv2
//	relationships:
//	  - relation: owner
//	    targettypes:
//	      - name: tenant
//	  - relation: foo_resource_get_rel
//	    targettypes:
//	      - name: user
//	        subjectidentifier: "*"
func (v *policy) createV2RoleResourceType() {
	role := ResourceType{
		Name:     v.p.RBAC.RoleResource.Name,
		IDPrefix: v.p.RBAC.RoleResource.IDPrefix,
	}

	// 1. create a relationship for role owners
	roleOwners := Relationship{
		Relation:    RoleOwnerRelation,
		TargetTypes: make([]types.TargetType, len(v.p.RBAC.RoleOwners)),
	}

	for i, owner := range v.p.RBAC.RoleOwners {
		roleOwners.TargetTypes[i] = types.TargetType{Name: owner}
	}

	// 2. create a list of relationships for all permissions and ownerships
	roleRel := make([]Relationship, 0, len(v.ac)+1)

	for _, action := range v.ac {
		targettypes := make([]types.TargetType, len(v.p.RBAC.RoleSubjectTypes))

		for j, subject := range v.p.RBAC.RoleSubjectTypes {
			targettypes[j] = types.TargetType{Name: subject, SubjectIdentifier: "*"}
		}

		roleRel = append(roleRel,
			Relationship{
				Relation:    action.Name + PermissionRelationSuffix,
				TargetTypes: targettypes,
			},
		)
	}

	// 3. create a role resource type containing all the relationships shown above
	roleRel = append(roleRel, roleOwners)

	role.Relationships = roleRel
	v.rt[role.Name] = role
}

// createRoleBindingResourceType creates a role-binding resource type contains
// a list of all the actions.
// The role-binding resources will be used to create a 3-way relationship
// between a resource, a subject and a role.
//
// this function effectively creates:
// 1. a resource type like this:
//
//	name: rolebinding
//	idprefix: permrbn
//	relationships:
//	  - relation: rolev2
//	    targettypes:
//	      - name: rolev2
//	  - relation: subject
//	    targettypes:
//	      - name: user
//	        subjectidentifier: "*"
//
// 2. a list of action-bindings representing permissions for all the actions in the policy
//
//	actionbindings:
//	  - actionname: foo_resource_get
//	    typename: rolebinding
//	    conditionsets:
//	      - conditions:
//	        - relationshipaction:
//	            relation: rolev2
//	            actionname: foo_resource_get_rel
//	      - conditions:
//	        - relationshipaction:
//	            relation: subject
//	   # ... other action bindings on the rolebinding resource for each
//	   # action defined in the policy
func (v *policy) createRoleBindingResourceType() {
	rolebinding := ResourceType{
		Name:     v.p.RBAC.RoleBindingResource.Name,
		IDPrefix: v.p.RBAC.RoleBindingResource.IDPrefix,
	}

	role := Relationship{
		Relation: RolebindingRoleRelation,
		TargetTypes: []types.TargetType{
			{Name: v.p.RBAC.RoleResource.Name},
		},
	}

	// 2. create relationship to subjects
	subjects := Relationship{
		Relation:    RolebindingSubjectRelation,
		TargetTypes: v.p.RBAC.RoleBindingSubjects,
	}

	// 3. create a list of action-bindings representing permissions for all the
	// actions in the policy
	actionbindings := make([]ActionBinding, 0, len(v.ac))

	for actionName := range v.ac {
		ab := ActionBinding{
			ActionName: actionName,
			TypeName:   v.p.RBAC.RoleBindingResource.Name,
			ConditionSets: []types.ConditionSet{
				{
					Conditions: []types.Condition{
						{
							RelationshipAction: &types.ConditionRelationshipAction{
								Relation:   RolebindingRoleRelation,
								ActionName: actionName + PermissionRelationSuffix,
							},
						},
					},
				},
				{
					Conditions: []types.Condition{
						{RelationshipAction: &types.ConditionRelationshipAction{Relation: RolebindingSubjectRelation}},
					},
				},
			},
		}

		actionbindings = append(actionbindings, ab)
	}

	v.bn = append(v.bn, actionbindings...)

	// 4. create role-binding resource type
	rolebinding.Relationships = []Relationship{role, subjects}
	v.rt[v.p.RBAC.RoleBindingResource.Name] = rolebinding
}

// expandRBACV2Relationships adds RBAC V2 relationships to all the resource
// types that has `ResourceRoleBindingV2` defined.
// Relationships like member_roles, available_roles, are created to support
// role inheritance, e.g., an org should be able to use roles defined by its
// parents.
// This function augments the resource types and effectively creating
// resource types like this:
//
// 1. for resource owners, `member_role` relationship is added
//
//	```diff
//	 resourcetypes:
//	   name: tenant
//	   idprefix: tnntten
//	   rolebindingv2:
//	     *rolesFromParent
//	   relationships:
//	     - relation: parent
//	       targettypes:
//	         - name: tenant_parent
//	     # ... other existing relationships
//	+	  - relation: member_role
//	+	    targettypes:
//	+	      - name: rolev2
//
//	```
//
// 2. for resources that inherit permissions from other resources, `available_roles`
// action is added
//
//	```diff
//	  actionbindings:
//	  # ... other existing action bindings
//	+   - actionname: available_roles
//	+     typename: tenant
//	+     conditions:
//	+       - relationshipaction:
//	+           relation: member_role
//	+       - relationshipaction:
//	+           relation: parent
//	+           actionname: available_roles
//	```
func (v *policy) expandRBACV2Relationships() {
	for name, resourceType := range v.rt {
		// not all roles are available for all resources, available roles are
		// the roles that a resource owners (if it is a role-owner) or inherited
		// from their owner or parent
		availableRoles := []Condition{}

		// if resource type is a role-owner, add the role-relationship to the
		// resource
		if _, ok := v.RBAC().RoleOwnersSet()[name]; ok {
			memberRoleRelation := Relationship{
				Relation: RoleOwnerMemberRoleRelation,
				TargetTypes: []types.TargetType{
					{Name: v.p.RBAC.RoleResource.Name},
				},
			}

			resourceType.Relationships = append(resourceType.Relationships, memberRoleRelation)

			// i.e. avail_role = member_role
			availableRoles = append(availableRoles, Condition{
				RelationshipAction: &ConditionRelationshipAction{
					Relation: RoleOwnerMemberRoleRelation,
				},
			})
		}

		// i.e. avail_role = from[0]->avail_role + from[1]->avail_role ...
		if resourceType.RoleBindingV2 != nil {
			for _, from := range resourceType.RoleBindingV2.InheritPermissionsFrom {
				availableRoles = append(availableRoles, Condition{
					RelationshipAction: &ConditionRelationshipAction{
						Relation:   from,
						ActionName: AvailableRolesList,
					},
				})
			}
		}

		// create available role permission
		if len(availableRoles) > 0 {
			action := ActionBinding{
				ActionName: AvailableRolesList,
				TypeName:   resourceType.Name,
				Conditions: availableRoles,
			}

			v.bn = append(v.bn, action)
		}

		v.rt[name] = resourceType
	}
}

func (v *policy) expandResourceTypes() {
	for name, resourceType := range v.rt {
		for i, rel := range resourceType.Relationships {
			targettypes := []types.TargetType{}

			for _, tt := range rel.TargetTypes {
				if u, ok := v.un[tt.Name]; ok {
					targettypes = append(targettypes, u.ResourceTypes...)
				} else {
					targettypes = append(targettypes, tt)
				}
			}

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

	if err := v.validateRoles(); err != nil {
		return fmt.Errorf("roles: %w", err)
	}

	return nil
}

func (v *policy) Schema() []types.ResourceType {
	typeMap := map[string]*types.ResourceType{}
	rbv2Actions := map[string][]types.Action{}

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

		action := types.Action{
			Name: actionName,
		}

		// rbac V2 actions
		res := v.rt[b.TypeName]

		for _, c := range b.Conditions {
			var conditions []types.Condition

			switch {
			case c.RoleBinding != nil:
				conditions = []types.Condition{
					{
						RelationshipAction: &types.ConditionRelationshipAction{
							Relation: actionName + PermissionRelationSuffix,
						},
						RoleBinding: &types.ConditionRoleBinding{},
					},
				}

				actionRel := types.ResourceTypeRelationship{
					Relation: actionName + PermissionRelationSuffix,
					Types:    []types.TargetType{{Name: RolebindingRoleRelation, SubjectRelation: RolebindingSubjectRelation}},
				}

				typeMap[b.TypeName].Relationships = append(typeMap[b.TypeName].Relationships, actionRel)
			case c.RoleBindingV2 != nil && res.RoleBindingV2 != nil:
				conditions = v.RBAC().CreateRoleBindingConditionsForAction(actionName, res.RoleBindingV2.InheritPermissionsFrom...)

				// add role-binding v2 conditions to the resource, if not exists
				if _, ok := rbv2Actions[b.TypeName]; !ok {
					rbv2Actions[b.TypeName] = v.RBAC().CreateRoleBindingActionsForResource(res.RoleBindingV2.InheritPermissionsFrom...)
				}
			default:
				conditions = []types.Condition{
					{
						RelationshipAction: (*types.ConditionRelationshipAction)(c.RelationshipAction),
					},
				}
			}

			action.Conditions = append(action.Conditions, conditions...)
		}

		action.ConditionSets = b.ConditionSets

		typeMap[b.TypeName].Actions = append(typeMap[b.TypeName].Actions, action)
	}

	for resType, actions := range rbv2Actions {
		typeMap[resType].Actions = append(typeMap[resType].Actions, actions...)
	}

	out := make([]types.ResourceType, 0, len(typeMap))
	for _, rt := range typeMap {
		out = append(out, *rt)
	}

	return out
}

// RBAC returns the RBAC configurations
func (v *policy) RBAC() *RBAC {
	return v.p.RBAC
}

func (v *policy) findRelationship(rels []Relationship, name string) bool {
	for _, rel := range rels {
		if rel.Relation == name {
			return true
		}
	}

	return false
}

func (v *policy) findActionBinding(actionName string, typeName string) bool {
	for _, bn := range v.bn {
		if bn.ActionName == actionName && bn.TypeName == typeName {
			return true
		}
	}

	return false
}
