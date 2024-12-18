package crdb

import (
	"embed"
)

// Migrations contains an embedded filesystem with all the sql migration files
// for cockroach db
//
//go:embed migrations/*.sql
var Migrations embed.FS
