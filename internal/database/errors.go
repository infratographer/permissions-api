package database

import (
	"errors"

	"github.com/lib/pq"
)

var (
	// ErrNoRoleFound is returned when no role is found when retrieving or deleting a role.
	ErrNoRoleFound = errors.New("role not found")

	// ErrRoleAlreadyExists is returned when creating a role which already has an existing record.
	ErrRoleAlreadyExists = errors.New("role already exists")

	// ErrRoleNameTaken is returned when the role name provided already exists under the same resource id.
	ErrRoleNameTaken = errors.New("role name already taken")
)

func pqIsRoleAlreadyExistsError(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code.Name() == "unique_violation" && pqErr.Constraint == "roles_pkey"
	}

	return false
}

func pqIsRoleNameTakenError(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code.Name() == "unique_violation" && pqErr.Constraint == "roles_resource_id_name"
	}

	return false
}
