package psql

import (
	"embed"
)

// Migrations contains an embedded filesystem with all the sql migration files
// for postgresql
//
//go:embed migrations/*.sql
var Migrations embed.FS
