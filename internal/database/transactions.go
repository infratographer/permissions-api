package database

import (
	"database/sql"
	"errors"

	"go.uber.org/zap"
)

// Transaction represents an in flight change being made to the database that must be committed or rolled back.
type Transaction[T any] struct {
	logger *zap.SugaredLogger
	tx     *sql.Tx

	Record T
}

// Commit completes the transaction and writes the changes to the database.
func (t *Transaction[T]) Commit() error {
	return t.tx.Commit()
}

// Rollback reverts the transaction and discards the changes from the database.
//
// To simplify rollbacks, logging has automatically been setup to log any errors produced if a rollback fails.
func (t *Transaction[T]) Rollback() error {
	err := t.tx.Rollback()
	if err != nil && !errors.Is(err, sql.ErrTxDone) {
		t.logger.Errorw("failed to rollback transaction", zap.Error(err))
	}

	return err
}

// newTransaction creates a new Transaction with the required fields.
func newTransaction[T any](logger *zap.SugaredLogger, tx *sql.Tx, record T) *Transaction[T] {
	return &Transaction[T]{
		logger: logger,
		tx:     tx,

		Record: record,
	}
}
