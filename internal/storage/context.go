package storage

import (
	"context"
	"database/sql"
)

// TransactionManager manages the state of sql transactions within a context
type TransactionManager interface {
	BeginContext(context.Context) (context.Context, error)
	CommitContext(context.Context) error
	RollbackContext(context.Context) error
}

type contextKey struct{}

var txKey contextKey

func beginTxContext(ctx context.Context, db DB) (context.Context, error) {
	tx, err := db.BeginTx(ctx, nil)

	if err != nil {
		return nil, err
	}

	out := context.WithValue(ctx, txKey, tx)

	return out, nil
}

func getContextTx(ctx context.Context) (*sql.Tx, error) {
	switch v := ctx.Value(txKey).(type) {
	case *sql.Tx:
		return v, nil
	case nil:
		return nil, ErrorMissingContextTx
	default:
		panic("unknown type for context transaction")
	}
}

func getContextDBQuery(ctx context.Context, def DBQuery) (DBQuery, error) {
	tx, err := getContextTx(ctx)

	switch err {
	case nil:
		return tx, nil
	case ErrorMissingContextTx:
		return def, nil
	default:
		return nil, err
	}
}

func commitContextTx(ctx context.Context) error {
	tx, err := getContextTx(ctx)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func rollbackContextTx(ctx context.Context) error {
	tx, err := getContextTx(ctx)
	if err != nil {
		return err
	}

	return tx.Rollback()
}

// BeginContext starts a new transaction.
func (e *engine) BeginContext(ctx context.Context) (context.Context, error) {
	return beginTxContext(ctx, e.DB)
}

// CommitContext commits the transaction in the provided context.
func (e *engine) CommitContext(ctx context.Context) error {
	return commitContextTx(ctx)
}

// RollbackContext rollsback the transaction in the provided context.
func (e *engine) RollbackContext(ctx context.Context) error {
	return rollbackContextTx(ctx)
}
