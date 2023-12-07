package storage

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

	// ErrMethodUnavailable is returned when the provided method is called is unavailable in the current environment.
	// For example there is nothing to commit after getting a role so calling Commit on a Role after retrieving it will return this error.
	ErrMethodUnavailable = errors.New("method unavailable")

	// ErrorMissingContextTx represents an error where no context transaction was provided.
	ErrorMissingContextTx = errors.New("no transaction provided in context")

	// ErrorInvalidContextTx represents an error where the given context transaction is of the wrong type.
	ErrorInvalidContextTx = errors.New("invalid type for transaction context")
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
