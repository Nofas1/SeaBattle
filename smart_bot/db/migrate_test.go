package db

import (
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func TestEmbeddedMigrationsHaveUniqueVersions(t *testing.T) {
	if _, err := iofs.New(migrationFiles, "migrations"); err != nil {
		t.Fatalf("initialize embedded migrations: %v", err)
	}
}
