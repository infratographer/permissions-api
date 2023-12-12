package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.infratographer.com/x/gidx"
)

// RoleService represents a service for managing roles.
type RoleService interface {
	GetRoleByID(ctx context.Context, id gidx.PrefixedID) (Role, error)
	GetResourceRoleByName(ctx context.Context, resourceID gidx.PrefixedID, name string) (Role, error)
	ListResourceRoles(ctx context.Context, resourceID gidx.PrefixedID) ([]Role, error)
	CreateRole(ctx context.Context, actorID gidx.PrefixedID, roleID gidx.PrefixedID, name string, resourceID gidx.PrefixedID) (Role, error)
	UpdateRole(ctx context.Context, actorID, roleID gidx.PrefixedID, name string) (Role, error)
	DeleteRole(ctx context.Context, roleID gidx.PrefixedID) (Role, error)
}

// Role represents a role in the database.
type Role struct {
	ID         gidx.PrefixedID
	Name       string
	ResourceID gidx.PrefixedID
	CreatedBy  gidx.PrefixedID
	UpdatedBy  gidx.PrefixedID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// GetRoleByID retrieves a role from the database by the provided prefixed ID.
// If no role exists an ErrRoleNotFound error is returned.
func (e *engine) GetRoleByID(ctx context.Context, id gidx.PrefixedID) (Role, error) {
	db, err := getContextDBQuery(ctx, e)
	if err != nil {
		return Role{}, err
	}

	var role Role

	err = db.QueryRowContext(ctx, `
		SELECT
			id,
			name,
			resource_id,
			created_by,
			updated_by,
			created_at,
			updated_at
		FROM roles
		WHERE id = $1
		`, id.String(),
	).Scan(
		&role.ID,
		&role.Name,
		&role.ResourceID,
		&role.CreatedBy,
		&role.UpdatedBy,
		&role.CreatedAt,
		&role.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Role{}, fmt.Errorf("%w: %s", ErrNoRoleFound, id.String())
		}

		return Role{}, fmt.Errorf("%w: %s", err, id.String())
	}

	return role, nil
}

// GetResourceRoleByName retrieves a role from the database by the provided resource ID and role name.
// If no role exists an ErrRoleNotFound error is returned.
func (e *engine) GetResourceRoleByName(ctx context.Context, resourceID gidx.PrefixedID, name string) (Role, error) {
	db, err := getContextDBQuery(ctx, e)
	if err != nil {
		return Role{}, err
	}

	var role Role

	err = db.QueryRowContext(ctx, `
		SELECT
			id,
			name,
			resource_id,
			created_by,
			updated_by,
			created_at,
			updated_at
		FROM roles
		WHERE
			resource_id = $1
			AND	name = $2
		`,
		resourceID.String(),
		name,
	).Scan(
		&role.ID,
		&role.Name,
		&role.ResourceID,
		&role.CreatedBy,
		&role.UpdatedBy,
		&role.CreatedAt,
		&role.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Role{}, fmt.Errorf("%w: %s", ErrNoRoleFound, name)
		}

		return Role{}, err
	}

	return role, nil
}

// ListResourceRoles retrieves all roles associated with the provided resource ID.
// If no roles are found an empty slice is returned.
func (e *engine) ListResourceRoles(ctx context.Context, resourceID gidx.PrefixedID) ([]Role, error) {
	db, err := getContextDBQuery(ctx, e)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT
			id,
			name,
			resource_id,
			created_by,
			updated_by,
			created_at,
			updated_at
		FROM roles
		WHERE
			resource_id = $1
		`,
		resourceID.String(),
	)

	if err != nil {
		return nil, err
	}

	var roles []Role

	for rows.Next() {
		var role Role

		if err := rows.Scan(&role.ID, &role.Name, &role.ResourceID, &role.CreatedBy, &role.UpdatedBy, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}

		roles = append(roles, role)
	}

	return roles, nil
}

// CreateRole creates a role with the provided details.
// If a role already exists with the given roleID an ErrRoleAlreadyExists error is returned.
// If a role already exists with the same name under the given resource ID then an ErrRoleNameTaken error is returned.
//
// This method must be called with a context returned from BeginContext.
// CommitContext or RollbackContext must be called afterwards if this method returns no error.
func (e *engine) CreateRole(ctx context.Context, actorID, roleID gidx.PrefixedID, name string, resourceID gidx.PrefixedID) (Role, error) {
	tx, err := getContextTx(ctx)
	if err != nil {
		return Role{}, err
	}

	var role Role

	err = tx.QueryRowContext(ctx, `
		INSERT
			INTO roles (id, name, resource_id, created_by, updated_by, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4, now(), now())
		RETURNING id, name, resource_id, created_by, updated_by, created_at, updated_at
		`, roleID.String(), name, resourceID.String(), actorID.String(),
	).Scan(
		&role.ID,
		&role.Name,
		&role.ResourceID,
		&role.CreatedBy,
		&role.UpdatedBy,
		&role.CreatedAt,
		&role.UpdatedAt,
	)

	if err != nil {
		if pqIsRoleAlreadyExistsError(err) {
			return Role{}, fmt.Errorf("%w: %s", ErrRoleAlreadyExists, roleID.String())
		}

		if pqIsRoleNameTakenError(err) {
			return Role{}, fmt.Errorf("%w: %s", ErrRoleNameTaken, name)
		}

		return Role{}, err
	}

	return role, nil
}

// UpdateRole updates an existing role.
// If changing the name and the new name results in a duplicate name error, an ErrRoleNameTaken error is returned.
//
// This method must be called with a context returned from BeginContext.
// CommitContext or RollbackContext must be called afterwards if this method returns no error.
func (e *engine) UpdateRole(ctx context.Context, actorID, roleID gidx.PrefixedID, name string) (Role, error) {
	tx, err := getContextTx(ctx)
	if err != nil {
		return Role{}, err
	}

	var role Role

	err = tx.QueryRowContext(ctx, `
		UPDATE roles SET name = $1, updated_by = $2, updated_at = now() WHERE id = $3
		RETURNING id, name, resource_id, created_by, updated_by, created_at, updated_at
		`, name, actorID.String(), roleID.String(),
	).Scan(
		&role.ID,
		&role.Name,
		&role.ResourceID,
		&role.CreatedBy,
		&role.UpdatedBy,
		&role.CreatedAt,
		&role.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Role{}, fmt.Errorf("%w: %s", ErrNoRoleFound, roleID.String())
		}

		if pqIsRoleNameTakenError(err) {
			return Role{}, fmt.Errorf("%w: %s", ErrRoleNameTaken, name)
		}

		return Role{}, err
	}

	return role, nil
}

// DeleteRole deletes the role for the id provided.
// If no rows are affected an ErrNoRoleFound error is returned.
//
// This method must be called with a context returned from BeginContext.
// CommitContext or RollbackContext must be called afterwards if this method returns no error.
func (e *engine) DeleteRole(ctx context.Context, roleID gidx.PrefixedID) (Role, error) {
	tx, err := getContextTx(ctx)
	if err != nil {
		return Role{}, err
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM roles WHERE id = $1`, roleID.String())
	if err != nil {
		return Role{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Role{}, err
	}

	if rowsAffected == 0 {
		return Role{}, ErrNoRoleFound
	}

	role := Role{
		ID: roleID,
	}

	return role, nil
}
