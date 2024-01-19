package storage

import (
	"embed"
)

// Migrations contains an embedded filesystem with all the sql migration files
//
//go:embed migrations/*.sql
var Migrations embed.FS
