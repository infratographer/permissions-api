// Package teststore is a testing helper package which initializes a new crdb database and runs migrations
// returning a new store which may be used during testing.
package teststore

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach-go/v2/testserver"
	_ "github.com/jackc/pgx/v5/stdlib" //nolint:revive // used for tests
	"github.com/pressly/goose/v3"

	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/storage/crdb"
)

// NewTestStorage creates a new permissions database instance for testing.
func NewTestStorage(t *testing.T) (storage.Storage, func()) {
	t.Helper()

	server, err := testserver.NewTestServer()
	if err != nil {
		t.Error(err)
		t.FailNow()

		return nil, func() {}
	}

	goose.SetBaseFS(crdb.Migrations)

	db, err := goose.OpenDBWithDriver("postgres", server.PGURL().String())
	if err != nil {
		t.Error(err)
		t.FailNow()

		return nil, func() {}
	}

	if err = goose.RunContext(context.Background(), "up", db, "migrations"); err != nil {
		t.Error(err)

		db.Close()

		t.FailNow()

		return nil, func() {}
	}

	return storage.New(db), func() { db.Close() }
}
