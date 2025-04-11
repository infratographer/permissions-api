package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.infratographer.com/permissions-api/internal/types"

	"go.infratographer.com/x/gidx"
)

// RoleBindingService represents a service for managing role bindings in the
// permissions API storage
type RoleBindingService interface {
	// ListResourceRoleBindings returns all role bindings for a given resource
	// an empty slice is returned if no role bindings are found
	ListResourceRoleBindings(ctx context.Context, resourceID gidx.PrefixedID) ([]types.RoleBinding, error)

	// ListManagerResourceRoleBindings returns all role bindings for a given resource and manager
	// an empty slice is returned if no role bindings are found
	ListManagerResourceRoleBindings(ctx context.Context, manager string, resourceID gidx.PrefixedID) ([]types.RoleBinding, error)

	// GetRoleBindingByID returns a role binding by its prefixed ID
	// an ErrRoleBindingNotFound error is returned if no role binding is found
	GetRoleBindingByID(ctx context.Context, id gidx.PrefixedID) (types.RoleBinding, error)

	// CreateRoleBinding creates a new role binding in the database
	// This method must be called with a context returned from BeginContext.
	// CommitContext or RollbackContext must be called afterwards if this method returns no error.
	CreateRoleBinding(ctx context.Context, actorID, rbID, resourceID gidx.PrefixedID, manager string) (types.RoleBinding, error)

	// UpdateRoleBinding updates a role binding in the database
	// Note that this method only updates the updated_at and updated_by fields
	// and do not provide a way to update the resource_id field.
	//
	// This method must be called with a context returned from BeginContext.
	// CommitContext or RollbackContext must be called afterwards if this method returns no error.
	UpdateRoleBinding(ctx context.Context, actorID, rbID gidx.PrefixedID) (types.RoleBinding, error)

	// DeleteRoleBinding deletes a role binding from the database
	// This method must be called with a context returned from BeginContext.
	// CommitContext or RollbackContext must be called afterwards if this method returns no error.
	DeleteRoleBinding(ctx context.Context, id gidx.PrefixedID) error

	// LockRoleBindingForUpdate locks a role binding record to be updated to ensure consistency.
	// If the role binding is not found, an ErrRoleBindingNotFound error is returned.
	LockRoleBindingForUpdate(ctx context.Context, id gidx.PrefixedID) error
}

func (e *engine) GetRoleBindingByID(ctx context.Context, id gidx.PrefixedID) (types.RoleBinding, error) {
	db, err := getContextDBQuery(ctx, e)
	if err != nil {
		return types.RoleBinding{}, err
	}

	var roleBinding types.RoleBinding

	err = db.QueryRowContext(ctx, `
		SELECT id, resource_id, manager, created_by, updated_by, created_at, updated_at
		FROM rolebindings WHERE id = $1
		`, id.String(),
	).Scan(
		&roleBinding.ID,
		&roleBinding.ResourceID,
		&roleBinding.Manager,
		&roleBinding.CreatedBy,
		&roleBinding.UpdatedBy,
		&roleBinding.CreatedAt,
		&roleBinding.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.RoleBinding{}, fmt.Errorf("%w: %s", ErrRoleBindingNotFound, id.String())
		}

		return types.RoleBinding{}, fmt.Errorf("%w: %s", err, id.String())
	}

	return roleBinding, nil
}

func (e *engine) ListResourceRoleBindings(ctx context.Context, resourceID gidx.PrefixedID) ([]types.RoleBinding, error) {
	db, err := getContextDBQuery(ctx, e)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, resource_id, manager, created_by, updated_by, created_at, updated_at
		FROM rolebindings WHERE resource_id = $1 ORDER BY created_at ASC
		`, resourceID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, resourceID.String())
	}
	defer rows.Close() //nolint:errcheck

	var roleBindings []types.RoleBinding

	for rows.Next() {
		var roleBinding types.RoleBinding

		err = rows.Scan(
			&roleBinding.ID,
			&roleBinding.ResourceID,
			&roleBinding.Manager,
			&roleBinding.CreatedBy,
			&roleBinding.UpdatedBy,
			&roleBinding.CreatedAt,
			&roleBinding.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", err, resourceID.String())
		}

		roleBindings = append(roleBindings, roleBinding)
	}

	return roleBindings, nil
}

func (e *engine) ListManagerResourceRoleBindings(ctx context.Context, manager string, resourceID gidx.PrefixedID) ([]types.RoleBinding, error) {
	db, err := getContextDBQuery(ctx, e)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, resource_id, manager, created_by, updated_by, created_at, updated_at
		FROM rolebindings WHERE manager = $1 AND resource_id = $1 ORDER BY created_at ASC
		`, manager, resourceID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, resourceID.String())
	}
	defer rows.Close() //nolint:errcheck

	var roleBindings []types.RoleBinding

	for rows.Next() {
		var roleBinding types.RoleBinding

		err = rows.Scan(
			&roleBinding.ID,
			&roleBinding.ResourceID,
			&roleBinding.Manager,
			&roleBinding.CreatedBy,
			&roleBinding.UpdatedBy,
			&roleBinding.CreatedAt,
			&roleBinding.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", err, resourceID.String())
		}

		roleBindings = append(roleBindings, roleBinding)
	}

	return roleBindings, nil
}

func (e *engine) CreateRoleBinding(ctx context.Context, actorID, rbID, resourceID gidx.PrefixedID, manager string) (types.RoleBinding, error) {
	tx, err := getContextTx(ctx)
	if err != nil {
		return types.RoleBinding{}, err
	}

	var rb types.RoleBinding

	err = tx.QueryRowContext(ctx, `
		INSERT INTO rolebindings (id, resource_id, manager, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4, $5, $5)
		RETURNING id, resource_id, manager, created_by, updated_by, created_at, updated_at
		`, rbID.String(), resourceID.String(), manager, actorID.String(), time.Now(),
	).Scan(
		&rb.ID,
		&rb.ResourceID,
		&rb.Manager,
		&rb.CreatedBy,
		&rb.UpdatedBy,
		&rb.CreatedAt,
		&rb.UpdatedAt,
	)
	if err != nil {
		return types.RoleBinding{}, fmt.Errorf("%w: %s", err, rbID.String())
	}

	return rb, nil
}

func (e *engine) UpdateRoleBinding(ctx context.Context, actorID, rbID gidx.PrefixedID) (types.RoleBinding, error) {
	tx, err := getContextTx(ctx)
	if err != nil {
		return types.RoleBinding{}, err
	}

	var rb types.RoleBinding

	err = tx.QueryRowContext(ctx, `
		UPDATE rolebindings
		SET updated_by = $1, updated_at = now()
		WHERE id = $2
		RETURNING id, resource_id, created_by, updated_by, created_at, updated_at
		`,
		actorID.String(), rbID.String(),
	).Scan(
		&rb.ID,
		&rb.ResourceID,
		&rb.CreatedBy,
		&rb.UpdatedBy,
		&rb.CreatedAt,
		&rb.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.RoleBinding{}, fmt.Errorf("%w: %s", ErrRoleBindingNotFound, rbID.String())
		}

		return types.RoleBinding{}, fmt.Errorf("%w: %s", err, rbID.String())
	}

	return rb, nil
}

func (e *engine) DeleteRoleBinding(ctx context.Context, id gidx.PrefixedID) error {
	tx, err := getContextTx(ctx)
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, `
		DELETE FROM rolebindings WHERE id = $1
		`, id.String(),
	)
	if err != nil {
		return fmt.Errorf("%w: %s", err, id.String())
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s", err, id.String())
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%w: %s", ErrRoleBindingNotFound, id.String())
	}

	return nil
}

func (e *engine) LockRoleBindingForUpdate(ctx context.Context, id gidx.PrefixedID) error {
	db, err := getContextDBQuery(ctx, e)
	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `SELECT 1 FROM rolebindings WHERE id = $1 FOR UPDATE`, id.String())
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRoleBindingNotFound
	}

	return nil
}

// buildBatchInClauseWithIDs is a helper function that builds an IN clause for
// a batch query with the provided prefixed IDs.
func (e *engine) buildBatchInClauseWithIDs(ids []gidx.PrefixedID) (clause string, args []any) {
	args = make([]any, len(ids))

	for i, id := range ids {
		fmtStr := "$%d"

		if i > 0 {
			fmtStr = ", $%d"
		}

		clause += fmt.Sprintf(fmtStr, i+1)
		args[i] = id.String()
	}

	return clause, args
}
