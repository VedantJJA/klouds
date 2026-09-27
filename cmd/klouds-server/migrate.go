package main

import (
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// newMigrate creates a new migrate instance for PostgreSQL.
func newMigrate(databaseURL string) (*migrate.Migrate, error) {
	return migrate.New("file://db/migrations", databaseURL)
}
