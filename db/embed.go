// Package db embeds the goose migrations.
package db

import "embed"

// Migrations contains the SQL files under migrations/.
//
//go:embed migrations/*.sql
var Migrations embed.FS
