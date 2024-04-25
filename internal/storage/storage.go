// Package storage interacts with the permissions-api database handling the metadata updates for roles and resources.
package storage

import (
	"context"
	"database/sql"

	"go.uber.org/zap"
)

// Storage defines the interface the engine exposes.
type Storage interface {
	RoleService
	RoleBindingService
	TransactionManager

	HealthCheck(ctx context.Context) error
}

// DB is the interface the database package requires from a database engine to run.
// *sql.DB implements these methods.
type DB interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	PingContext(ctx context.Context) error

	DBQuery
}

// DBQuery are required methods for querying the database.
type DBQuery interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type engine struct {
	DB
	logger *zap.SugaredLogger
}

// HealthCheck calls the underlying databases PingContext to check that the database is alive and accepting connections.
func (e *engine) HealthCheck(ctx context.Context) error {
	return e.PingContext(ctx)
}

// New creates a new storage engine using the provided underlying DB.
func New(db DB, options ...Option) Storage {
	s := &engine{
		DB:     db,
		logger: zap.NewNop().Sugar(),
	}

	for _, opt := range options {
		opt(s)
	}

	return s
}
