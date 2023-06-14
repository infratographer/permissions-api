package iapl

import (
	"fmt"

	"go.infratographer.com/permissions-api/internal/types"
)

// PolicyDocument represents a partial authorization policy.
type PolicyDocument struct {
	ResourceTypes  []ResourceType
	Unions         []Union
	Actions        []Action
	ActionBindings []ActionBinding
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
}

// Union represents a named union of multiple concrete resource types.
type Union struct {
	Name              string
	ResourceTypeNames []string
}

// Action represents an action that can be taken in an authorization policy.
type Action struct {
	Name string
}

// ActionBinding represents a binding of an action to a resource type or union.
type ActionBinding struct {
	ActionName string
	TypeName   string
	Conditions []Condition
}

// Condition represents a necessary condition for performing an action.
type Condition struct {
	RoleBinding        *ConditionRoleBinding
	RelationshipAction *ConditionRelationshipAction
}

// ConditionRoleBinding represents a condition where a role binding is necessary to perform an action.
type ConditionRoleBinding struct {
}

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
		rt: rt,
		un: un,
		ac: ac,
		p:  p,
	}

	return &out
}

func (v *policy) validateUnions() error {
	for _, typeAlias := range v.p.Unions {
		if _, ok := v.rt[typeAlias.Name]; ok {
			return fmt.Errorf("%s: %w", typeAlias.Name, ErrorInvalidAlias)
		}

		for _, rtName := range typeAlias.ResourceTypeNames {
			if _, ok := v.rt[rtName]; !ok {
				return fmt.Errorf("%s: resourceTypeNames: %s: %w", typeAlias.Name, rtName, ErrorUnknownType)
			}
		}
	}

	return nil
}

func (v *policy) validateResourceTypes() error {
	for _, resourceType := range v.p.ResourceTypes {
		for _, rel := range resourceType.Relationships {
			for _, name := range rel.TargetTypeNames {
				if _, ok := v.rt[name]; !ok {
					return fmt.Errorf("%s: relationships: %s: %w", resourceType.Name, name, ErrorUnknownType)
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
					TypeName:   typeName,
					ActionName: bn.ActionName,
					Conditions: bn.Conditions,
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

func (v *policy) expandResourceTypes() {
	for name, resourceType := range v.rt {
		for i, rel := range resourceType.Relationships {
			var typeNames []string

			for _, typeName := range rel.TargetTypeNames {
				if u, ok := v.un[typeName]; ok {
					typeNames = append(typeNames, u.ResourceTypeNames...)
				} else {
					typeNames = append(typeNames, typeName)
				}
			}

			resourceType.Relationships[i].TargetTypeNames = typeNames
		}

		v.rt[name] = resourceType
	}
}

func (v *policy) Validate() error {
	v.expandActionBindings()
	v.expandResourceTypes()

	if err := v.validateUnions(); err != nil {
		return fmt.Errorf("typeAliases: %w", err)
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
			Name: rt.Name,
		}

		for _, rel := range rt.Relationships {
			outRel := types.ResourceTypeRelationship{
				Relation: rel.Relation,
				Types:    rel.TargetTypeNames,
			}

			out.Relationships = append(out.Relationships, outRel)
		}

		typeMap[n] = &out
	}

	for _, b := range v.bn {
		action := types.Action{
			Name: b.ActionName,
		}

		for _, c := range b.Conditions {
			condition := types.Condition{
				RoleBinding:        (*types.ConditionRoleBinding)(c.RoleBinding),
				RelationshipAction: (*types.ConditionRelationshipAction)(c.RelationshipAction),
			}

			action.Conditions = append(action.Conditions, condition)
		}

		typeMap[b.TypeName].Actions = append(typeMap[b.TypeName].Actions, action)
	}

	out := make([]types.ResourceType, len(v.p.ResourceTypes))
	for i, rt := range v.p.ResourceTypes {
		out[i] = *typeMap[rt.Name]
	}

	return out
}
