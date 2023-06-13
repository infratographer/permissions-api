// Package types exposes domain types for permissions-api.
package types

import "github.com/google/uuid"

// Role is a collection of permissions.
type Role struct {
	ID      uuid.UUID
	Actions []string
}

// ResourceTypeRelationship is a relationship for a resource type.
type ResourceTypeRelationship struct {
	Relation string
	Types    []string
}

type ConditionRoleBinding struct{}

type ConditionRelationshipAction struct {
	Relation   string
	ActionName string
}

type Condition struct {
	RoleBinding        *ConditionRoleBinding
	RelationshipAction *ConditionRelationshipAction
}

type Action struct {
	Name       string
	Conditions []Condition
}

// ResourceType defines a type of resource managed by the api
type ResourceType struct {
	Name          string
	Relationships []ResourceTypeRelationship
	Actions       []Action
}

// Resource is the object to be acted upon by an subject
type Resource struct {
	Type string
	ID   uuid.UUID
}

// Relationship represents a named association between a resource and a subject.
type Relationship struct {
	Resource Resource
	Relation string
	Subject  Resource
}
