package storage

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
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

const (
	// Postgres error codes: https://www.postgresql.org/docs/11/errcodes-appendix.html
	pgErrCodeUniqueViolation = "23505"

	pqIndexRolesPrimaryKey     = "roles_pkey"
	pqIndexRolesResourceIDName = "roles_resource_id_name"
)

// pqIsRoleAlreadyExistsError checks that the provided error is a postgres error.
// If so, checks if postgres threw a unique_violation error on the roles primary key index.
// If postgres has raised a unique violation error on this index it means a record already exists
// with a matching primary key (role id).
func pqIsRoleAlreadyExistsError(err error) bool {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == pgErrCodeUniqueViolation && pgErr.ConstraintName == pqIndexRolesPrimaryKey
	}

	return false
}

// pqIsRoleNameTakenError checks that the provided error is a postgres error.
// If so, checks if postgres threw a unique_violation error on the roles resource id name index.
// If postgres has raised a unique violation error on this index it means a record already exists
// with the same resource id and role name combination.
func pqIsRoleNameTakenError(err error) bool {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == pgErrCodeUniqueViolation && pgErr.ConstraintName == pqIndexRolesResourceIDName
	}

	return false
}
