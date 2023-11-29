package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.infratographer.com/x/gidx"
	"go.uber.org/zap"
)

// Role represents a role in the database.
type Role struct {
	ID         gidx.PrefixedID
	Name       string
	ResourceID gidx.PrefixedID
	CreatorID  gidx.PrefixedID
	CreatedAt  time.Time
	UpdatedAt  time.Time

	logger   *zap.SugaredLogger
	commit   func() error
	rollback func() error
}

// Commit calls commit on the transaction if the role has been created within a transaction.
// If not the method returns an ErrMethodUnavailable error.
func (r *Role) Commit() error {
	if r.commit == nil {
		return ErrMethodUnavailable
	}

	return r.commit()
}

// Rollback calls rollback on the transaction if the role has been created within a transaction.
// If not the method returns an ErrMethodUnavailable error.
//
// To simplify rollbacks, logging has automatically been setup to log any errors produced if a rollback fails.
func (r *Role) Rollback() error {
	if r.rollback == nil {
		return ErrMethodUnavailable
	}

	err := r.rollback()
	if err != nil && !errors.Is(err, sql.ErrTxDone) {
		r.logger.Errorw("failed to rollback role", "role_id", r.ID, zap.Error(err))
	}

	return err
}

// GetRoleByID retrieves a role from the database by the provided prefixed ID.
// If no role exists an ErrRoleNotFound error is returned.
func (db *database) GetRoleByID(ctx context.Context, id gidx.PrefixedID) (*Role, error) {
	var role Role

	err := db.QueryRowContext(ctx, `
		SELECT
			id,
			name,
			resource_id,
			creator_id,
			created_at,
			updated_at
		FROM roles
		WHERE id = $1
		`, id.String(),
	).Scan(
		&role.ID,
		&role.Name,
		&role.ResourceID,
		&role.CreatorID,
		&role.CreatedAt,
		&role.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", ErrNoRoleFound, id.String())
		}

		return nil, fmt.Errorf("%w: %s", err, id.String())
	}

	return &role, nil
}

// GetResourceRoleByName retrieves a role from the database by the provided resource ID and role name.
// If no role exists an ErrRoleNotFound error is returned.
func (db *database) GetResourceRoleByName(ctx context.Context, resourceID gidx.PrefixedID, name string) (*Role, error) {
	var role Role

	err := db.QueryRowContext(ctx, `
		SELECT
			id,
			name,
			resource_id,
			creator_id,
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
		&role.CreatorID,
		&role.CreatedAt,
		&role.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", ErrNoRoleFound, name)
		}

		return nil, err
	}

	return &role, nil
}

// ListResourceRoles retrieves all roles associated with the provided resource ID.
// If no roles are found an empty slice is returned.
func (db *database) ListResourceRoles(ctx context.Context, resourceID gidx.PrefixedID) ([]*Role, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			id,
			name,
			resource_id,
			creator_id,
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

	var roles []*Role

	for rows.Next() {
		var role Role

		if err := rows.Scan(&role.ID, &role.Name, &role.ResourceID, &role.CreatorID, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}

		roles = append(roles, &role)
	}

	return roles, nil
}

// CreateRole creates a role with the provided details returning the new Role entry.
// If a role already exists with the given roleID an ErrRoleAlreadyExists error is returned.
// If a role already exists with the same name under the given resource ID then an ErrRoleNameTaken error is returned.
func (db *database) CreateRole(ctx context.Context, actorID, roleID gidx.PrefixedID, name string, resourceID gidx.PrefixedID) (*Role, error) {
	var role Role

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	err = tx.QueryRowContext(ctx, `
		INSERT
			INTO roles (id, name, resource_id, creator_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, now(), now())
		RETURNING id, name, resource_id, creator_id, created_at, updated_at
		`, roleID.String(), name, resourceID.String(), actorID.String(),
	).Scan(
		&role.ID,
		&role.Name,
		&role.ResourceID,
		&role.CreatorID,
		&role.CreatedAt,
		&role.UpdatedAt,
	)

	if err != nil {
		if pqIsRoleAlreadyExistsError(err) {
			return nil, fmt.Errorf("%w: %s", ErrRoleAlreadyExists, roleID.String())
		}

		if pqIsRoleNameTakenError(err) {
			return nil, fmt.Errorf("%w: %s", ErrRoleNameTaken, name)
		}

		return nil, err
	}

	role.logger = db.logger.Named("role")
	role.commit = tx.Commit
	role.rollback = tx.Rollback

	return &role, nil
}

// UpdateRole updates an existing role if one exists.
// If no role already exists, a new role is created in the same way as CreateRole.
// If changing the name and the new name results in a duplicate name error, an ErrRoleNameTaken error is returned.
func (db *database) UpdateRole(ctx context.Context, actorID, roleID gidx.PrefixedID, name string, resourceID gidx.PrefixedID) (*Role, error) {
	var role Role

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	err = tx.QueryRowContext(ctx, `
		INSERT INTO roles (id, name, resource_id, creator_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, now(), now())
		ON CONFLICT (id) DO UPDATE
			SET (name, resource_id, updated_at) = (excluded.name, excluded.resource_id, excluded.updated_at)
		RETURNING id, name, resource_id, creator_id, created_at, updated_at
		`, roleID.String(), name, resourceID.String(), actorID.String(),
	).Scan(
		&role.ID,
		&role.Name,
		&role.ResourceID,
		&role.CreatorID,
		&role.CreatedAt,
		&role.UpdatedAt,
	)

	if err != nil {
		if pqIsRoleNameTakenError(err) {
			return nil, fmt.Errorf("%w: %s", ErrRoleNameTaken, name)
		}

		return nil, err
	}

	role.logger = db.logger.Named("role")
	role.commit = tx.Commit
	role.rollback = tx.Rollback

	return &role, nil
}

// DeleteRole deletes the role id provided, if no rows are affected an ErrNoRoleFound error is returned.
func (db *database) DeleteRole(ctx context.Context, roleID gidx.PrefixedID) (*Role, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM roles WHERE id = $1`, roleID.String())
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected != 1 {
		return nil, ErrNoRoleFound
	}

	return &Role{
		ID:       roleID,
		logger:   db.logger.Named("role"),
		commit:   tx.Commit,
		rollback: tx.Rollback,
	}, nil
}
