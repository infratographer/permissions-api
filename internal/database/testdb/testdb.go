// Package testdb is a testing helper package which initializes a new crdb database and runs migrations
// returning a new database which may be used during testing.
package testdb

import (
	"testing"

	"github.com/cockroachdb/cockroach-go/v2/testserver"
	"github.com/pressly/goose/v3"

	dbm "go.infratographer.com/permissions-api/db"
	"go.infratographer.com/permissions-api/internal/database"
)

// NewTestDatabase creates a new permissions database instance for testing.
func NewTestDatabase(t *testing.T) (database.Database, func()) {
	t.Helper()

	server, err := testserver.NewTestServer()
	if err != nil {
		t.Error(err)
		t.FailNow()

		return nil, func() {}
	}

	goose.SetBaseFS(dbm.Migrations)

	db, err := goose.OpenDBWithDriver("postgres", server.PGURL().String())
	if err != nil {
		t.Error(err)
		t.FailNow()

		return nil, func() {}
	}

	if err = goose.Run("up", db, "migrations"); err != nil {
		t.Error(err)

		db.Close()

		t.FailNow()

		return nil, func() {}
	}

	return database.NewDatabase(db), func() { db.Close() }
}
