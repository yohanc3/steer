// Package storage initializes SQLite and applies embedded migrations.
package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	// Open the driver connection before applying connection-local SQLite settings.
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Verify the file is reachable before returning a usable repository database.
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	// Enforce relational integrity and bounded lock waits for the single connection.
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure sqlite: %w", err)
		}
	}
	// Bring an existing database forward before any repository method can query it.
	if err := migrateUp(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrateUp(db *sql.DB) error {
	// Read migration files from the embedded binary rather than the working directory.
	sourceDriver, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}
	// Adapt the existing SQLite handle for golang-migrate.
	databaseDriver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("create migration database driver: %w", err)
	}
	// Apply all unapplied versions and treat an already-current database as success.
	migrator, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", databaseDriver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
