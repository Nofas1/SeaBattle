package db

import (
	"embed"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
)

//go:embed migrations\*.sql
var migrationFiles embed.FS

type Migrator struct {
    log  *slog.Logger
    conn *sqlx.DB
}

func NewMigrator(log *slog.Logger, conn *sqlx.DB) *Migrator {
    return &Migrator{log: log, conn: conn}
}

func (mg *Migrator) Migrate() error {
    mg.log.Debug("running migration")

    files, err := iofs.New(migrationFiles, "migrations")
    if err != nil {
        return err
    }

    driver, err := pgx.WithInstance(mg.conn.DB, &pgx.Config{})
    if err != nil {
        return err
    }

    m, err := migrate.NewWithInstance("iofs", files, "pgx", driver)
    if err != nil {
        return err
    }

    err = m.Up()
    if err != nil && err != migrate.ErrNoChange {
        mg.log.Error("migration failed", "error", err)
        return err
    }

    mg.log.Debug("migration finished")
    return nil
}
