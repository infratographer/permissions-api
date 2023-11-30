// Package database interacts with the permissions-api database handling the metadata updates for roles and resources.
package database

import (
	"context"
	"database/sql"

	"go.infratographer.com/x/gidx"
	"go.uber.org/zap"
)

// Database defines the interface the database exposes.
type Database interface {
	GetRoleByID(ctx context.Context, id gidx.PrefixedID) (*Role, error)
	GetResourceRoleByName(ctx context.Context, resourceID gidx.PrefixedID, name string) (*Role, error)
	ListResourceRoles(ctx context.Context, resourceID gidx.PrefixedID) ([]*Role, error)
	CreateRoleTransaction(ctx context.Context, actorID gidx.PrefixedID, roleID gidx.PrefixedID, name string, resourceID gidx.PrefixedID) (*Transaction[*Role], error)
	UpdateRoleTransaction(ctx context.Context, actorID gidx.PrefixedID, roleID gidx.PrefixedID, name string, resourceID gidx.PrefixedID) (*Transaction[*Role], error)
	DeleteRoleTransaction(ctx context.Context, roleID gidx.PrefixedID) (*Transaction[*Role], error)
	HealthCheck(ctx context.Context) error
}

// DB is the interface the database package requires from a database engine to run.
// *sql.DB implements these methods.
type DB interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	PingContext(ctx context.Context) error
}

type database struct {
	DB
	logger *zap.SugaredLogger
}

// HealthCheck calls the underlying databases PingContext to check that the database is alive and accepting connections.
func (db *database) HealthCheck(ctx context.Context) error {
	return db.PingContext(ctx)
}

// NewDatabase creates a new Database using the provided underlying DB.
func NewDatabase(db DB, options ...Option) Database {
	d := &database{
		DB:     db,
		logger: zap.NewNop().Sugar(),
	}

	for _, opt := range options {
		opt(d)
	}

	return d
}
