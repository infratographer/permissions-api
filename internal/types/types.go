// Package types exposes domain types for permissions-api.
package types

import "github.com/google/uuid"

// RoleTemplate is a template for a role
type RoleTemplate struct {
	Actions []string
}

// Role is a collection of permissions
type Role struct {
	ID      uuid.UUID
	Actions []string
}

// ResourceRelationship is a relationship for a resource type
type ResourceRelationship struct {
	Name     string
	Type     string
	Optional bool
}

// ResourceType defines a type of resource managed by the api
type ResourceType struct {
	Name          string
	Relationships []ResourceRelationship
	TenantActions []string
}

// Resource is the object to be acted upon by an subject
type Resource struct {
	Type string
	ID   uuid.UUID
}
