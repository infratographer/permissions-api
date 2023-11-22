// Package database interacts with the permissions-api database handling the metadata updates for roles and resources.
package database

import (
	"context"
	"database/sql"

	"go.infratographer.com/x/gidx"
)

// Database defines the interface the database exposes.
type Database interface {
	GetRoleByID(ctx context.Context, id gidx.PrefixedID) (*Role, error)
	GetResourceRoleByName(ctx context.Context, resourceID gidx.PrefixedID, name string) (*Role, error)
	ListResourceRoles(ctx context.Context, resourceID gidx.PrefixedID) ([]*Role, error)
	CreateRole(ctx context.Context, actorID gidx.PrefixedID, roleID gidx.PrefixedID, name string, resourceID gidx.PrefixedID) (*Role, error)
	UpdateRole(ctx context.Context, actorID gidx.PrefixedID, roleID gidx.PrefixedID, name string, resourceID gidx.PrefixedID) (*Role, error)
	DeleteRole(ctx context.Context, roleID gidx.PrefixedID) (*Role, error)
}

// DB is the interface the database package requires from a database engine to run.
// *sql.DB implements these methods.
type DB interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type database struct {
	DB
}

// NewDatabase creates a new Database using the provided underlying DB.
func NewDatabase(db DB) Database {
	return &database{
		DB: db,
	}
}
