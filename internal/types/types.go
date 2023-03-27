// Package type exposes domain types for permissions-api.
package types

import "github.com/google/uuid"

type RoleTemplate struct {
	Actions []string
}

type Role struct {
	ID      *uuid.UUID
	Actions []string
}

type ResourceRelationship struct {
	Name     string
	Type     string
	Optional bool
}

type ResourceType struct {
	Name          string
	Relationships []ResourceRelationship
	TenantActions []string
}

type Resource struct {
	Type string
	ID   *uuid.UUID
}
