// Package types exposes domain types for permissions-api.
package types

import (
	"time"

	"go.infratographer.com/x/gidx"
)

// Role is a collection of permissions.
type Role struct {
	ID      gidx.PrefixedID
	Name    string
	Actions []string

	ResourceID gidx.PrefixedID
	CreatedBy  gidx.PrefixedID
	UpdatedBy  gidx.PrefixedID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TargetType represents a relationship target, as defined in spiceDB's schema
// reference: https://authzed.com/docs/reference/schema-lang#relations
type TargetType struct {
	Name              string
	SubjectIdentifier string
	SubjectRelation   string
}

// ResourceTypeRelationship is a relationship for a resource type.
type ResourceTypeRelationship struct {
	Relation string
	Types    []TargetType
}

// ConditionRoleBinding represents a condition where a role binding is necessary to perform an action.
type ConditionRoleBinding struct{}

// ConditionRoleBindingV2 represents a condition where a role binding is necessary to perform an action.
// This is the new version of the condition, and it is used to support the new role binding resource type.
type ConditionRoleBindingV2 struct{}

// ConditionRelationshipAction represents a condition where an action must be able to be performed
// on another resource along a relation to perform an action.
type ConditionRelationshipAction struct {
	Relation   string
	ActionName string
}

// Condition represents a required condition for performing an action.
type Condition struct {
	RoleBinding        *ConditionRoleBinding
	RoleBindingV2      *ConditionRoleBindingV2
	RelationshipAction *ConditionRelationshipAction
}

// ConditionSet is a set of conditions that must be met for the action to be performed.
type ConditionSet struct {
	Conditions []Condition
}

// Action represents a named thing a subject can do.
type Action struct {
	Name          string
	Conditions    []Condition
	ConditionSets []ConditionSet
}

// ResourceType defines a type of resource managed by the api
type ResourceType struct {
	Name          string
	IDPrefix      string
	Relationships []ResourceTypeRelationship
	Actions       []Action
}

// Resource is the object to be acted upon by an subject
type Resource struct {
	Type string
	ID   gidx.PrefixedID
}

// RoleBindingSubject is the object that represents the subject of a role binding.
type RoleBindingSubject struct {
	SubjectResource Resource
}

// Relationship represents a named association between a resource and a subject.
type Relationship struct {
	Resource Resource
	Relation string
	Subject  Resource
}

// RoleBinding represents a role binding between a role and a resource.
type RoleBinding struct {
	ID         gidx.PrefixedID
	ResourceID gidx.PrefixedID
	RoleID     gidx.PrefixedID
	SubjectIDs []gidx.PrefixedID

	CreatedBy gidx.PrefixedID
	UpdatedBy gidx.PrefixedID
	CreatedAt time.Time
	UpdatedAt time.Time
}
