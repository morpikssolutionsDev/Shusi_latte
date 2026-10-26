// Package db holds the SQL migrations and sqlc query sources.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
