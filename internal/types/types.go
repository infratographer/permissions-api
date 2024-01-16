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

// ResourceTypeRelationship is a relationship for a resource type.
type ResourceTypeRelationship struct {
	Relation string
	Types    []string
}

// ConditionRoleBinding represents a condition where a role binding is necessary to perform an action.
type ConditionRoleBinding struct{}

// ConditionRelationshipAction represents a condition where an action must be able to be performed
// on another resource along a relation to perform an action.
type ConditionRelationshipAction struct {
	Relation   string
	ActionName string
}

// Condition represents a required condition for performing an action.
type Condition struct {
	RoleBinding        *ConditionRoleBinding
	RelationshipAction *ConditionRelationshipAction
}

// Action represents a named thing a subject can do.
type Action struct {
	Name       string
	Conditions []Condition
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

// Relationship represents a named association between a resource and a subject.
type Relationship struct {
	Resource Resource
	Relation string
	Subject  Resource
}
