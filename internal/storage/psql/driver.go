package psql

import (
	"database/sql"
	"fmt"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v5" // Register pgx driver.
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// NewDB will open a connection to the database based on the config provided
func NewDB(cfg Config, tracing bool) (*sql.DB, error) {
	dbDriverName := "pgx"

	var err error

	if tracing {
		// Register an OTel SQL driver
		dbDriverName, err = otelsql.Register(dbDriverName,
			otelsql.WithAttributes(semconv.DBSystemPostgreSQL))
		if err != nil {
			return nil, fmt.Errorf("failed creating sql tracer: %w", err)
		}
	}

	db, err := sql.Open(dbDriverName, cfg.GetURI())
	if err != nil {
		return nil, fmt.Errorf("failed connecting to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed verifying database connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.Connections.MaxOpen)
	db.SetMaxIdleConns(cfg.Connections.MaxIdle)
	db.SetConnMaxIdleTime(cfg.Connections.MaxLifetime)

	return db, nil
}
