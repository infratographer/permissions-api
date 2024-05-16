package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"go.infratographer.com/x/gidx"
)

// ZedTokenService represents a service for getting and updating ZedTokens for resources.
type ZedTokenService interface {
	GetLatestZedToken(ctx context.Context, ids ...gidx.PrefixedID) (string, error)
	UpsertZedToken(ctx context.Context, id gidx.PrefixedID, zedToken string) error
}

func (e *engine) GetLatestZedToken(ctx context.Context, ids ...gidx.PrefixedID) (string, error) {
	db, err := getContextDBQuery(ctx, e)
	if err != nil {
		return "", err
	}

	inClause, args := e.buildBatchInClauseWithIDs(ids)
	q := fmt.Sprintf(`
		SELECT zedtoken
		FROM zedtokens
		WHERE resource_id IN (%s)
		AND current_timestamp() < expires_at
		ORDER BY created_at DESC
		LIMIT 1
	`, inClause)

	var out string

	err = db.QueryRowContext(ctx, q, args...).Scan(&out)

	switch {
	case err == nil:
		return out, nil
	case errors.Is(err, sql.ErrNoRows):
		return "", nil
	default:
		return "", err
	}
}

func (e *engine) UpsertZedToken(ctx context.Context, id gidx.PrefixedID, zedToken string) error {
	tx, err := getContextTx(ctx)
	if err != nil {
		return err
	}

	queryStub := `
		UPSERT INTO zedtokens (resource_id, zedtoken, created_at, expires_at)
		VALUES ($1, $2, current_timestamp(), current_timestamp() + (INTERVAL '1 hour'))
	`
	if _, err := tx.ExecContext(ctx, queryStub, id, zedToken); err != nil {
		return err
	}

	return nil
}
