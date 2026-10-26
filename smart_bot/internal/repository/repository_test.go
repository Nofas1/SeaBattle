package repository

import "testing"

func TestLoadDatabaseURLUsesEnvironmentWithoutLocalDotEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://bot:secret@postgres:5432/sea_battle")

	databaseURL, err := loadDatabaseURL()
	if err != nil {
		t.Fatalf("loadDatabaseURL() error = %v", err)
	}
	if databaseURL != "postgres://bot:secret@postgres:5432/sea_battle" {
		t.Fatalf("loadDatabaseURL() = %q; expected URL from environment", databaseURL)
	}
}

func TestLoadDatabaseURLRejectsMissingEnvironmentValue(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	if _, err := loadDatabaseURL(); err == nil {
		t.Fatal("loadDatabaseURL() succeeded without DATABASE_URL")
	}
}
