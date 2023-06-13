package iapl

import (
	"fmt"

	"go.infratographer.com/permissions-api/internal/types"
)

// PolicyDocument represents a partial authorization policy.
type PolicyDocument struct {
	ResourceTypes  []ResourceType
	TypeAliases    []TypeAlias
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
	Relation       string
	TargetTypeName string
}

// TypeAlias represents a named alias to multiple concrete resource types.
type TypeAlias struct {
	Name              string
	ResourceTypeNames []string
}

// Action represents an action that can be taken in an authorization policy.
type Action struct {
	Name string
}

// ActionBinding represents a binding of an action to a resource type.
type ActionBinding struct {
	ActionName       string
	ResourceTypeName string
	Conditions       []Condition
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
	ta map[string]TypeAlias
	ac map[string]Action
	rb map[string]map[string]struct{}
	p  PolicyDocument
}

// NewPolicy creates a policy from the given policy document.
func NewPolicy(p PolicyDocument) Policy {
	rt := make(map[string]ResourceType, len(p.ResourceTypes))
	for _, r := range p.ResourceTypes {
		rt[r.Name] = r
	}

	ta := make(map[string]TypeAlias, len(p.TypeAliases))
	for _, t := range p.TypeAliases {
		ta[t.Name] = t
	}

	ac := make(map[string]Action, len(p.Actions))
	for _, a := range p.Actions {
		ac[a.Name] = a
	}

	rb := make(map[string]map[string]struct{}, len(p.ResourceTypes))
	for _, ab := range p.ActionBindings {
		b, ok := rb[ab.ResourceTypeName]
		if !ok {
			b = make(map[string]struct{})
			rb[ab.ResourceTypeName] = b
		}

		b[ab.ActionName] = struct{}{}
	}

	out := policy{
		rt: rt,
		ta: ta,
		ac: ac,
		rb: rb,
		p:  p,
	}

	return &out
}

func (v *policy) validateTypeAliases() error {
	for _, typeAlias := range v.p.TypeAliases {
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
			if _, ok := v.rt[rel.TargetTypeName]; ok {
				continue
			}

			if _, ok := v.ta[rel.TargetTypeName]; ok {
				continue
			}

			return fmt.Errorf("%s: relationships: %s: %w", resourceType.Name, rel.TargetTypeName, ErrorUnknownType)
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

	var typeNames []string

	if _, ok := v.rt[rel.TargetTypeName]; ok {
		typeNames = []string{
			rel.TargetTypeName,
		}
	} else {
		typeNames = v.ta[rel.TargetTypeName].ResourceTypeNames
	}

	for _, tn := range typeNames {
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
	for i, binding := range v.p.ActionBindings {
		if _, ok := v.ac[binding.ActionName]; !ok {
			return fmt.Errorf("%d: %s: %w", i, binding.ActionName, ErrorUnknownAction)
		}

		rt, ok := v.rt[binding.ResourceTypeName]
		if !ok {
			return fmt.Errorf("%d: %s: %w", i, binding.ResourceTypeName, ErrorUnknownType)
		}

		if err := v.validateConditions(rt, binding.Conditions); err != nil {
			return fmt.Errorf("%d: conditions: %w", i, err)
		}
	}

	return nil
}

func (v *policy) Validate() error {
	if err := v.validateTypeAliases(); err != nil {
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
			}

			if alias, ok := v.ta[rel.TargetTypeName]; ok {
				outRel.Types = alias.ResourceTypeNames
			} else {
				outRel.Types = []string{
					rel.TargetTypeName,
				}
			}

			out.Relationships = append(out.Relationships, outRel)
		}

		typeMap[n] = &out
	}

	for _, b := range v.p.ActionBindings {
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
		typeMap[b.ResourceTypeName].Actions = append(typeMap[b.ResourceTypeName].Actions, action)
	}

	out := make([]types.ResourceType, len(v.p.ResourceTypes))
	for i, rt := range v.p.ResourceTypes {
		out[i] = *typeMap[rt.Name]
	}

	return out
}
