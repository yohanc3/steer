package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAppliesMigrationsIdempotently(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "steer.sqlite")

	for range 2 {
		db, err := Open(context.Background(), databasePath)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		var tableCount int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master
			WHERE type = 'table' AND name IN ('users', 'telegram_accounts', 'email_verification_codes', 'budget_limits', 'enrollments', 'transactions', 'jobs')`).Scan(&tableCount); err != nil {
			db.Close()
			t.Fatalf("query schema: %v", err)
		}
		if tableCount != 7 {
			db.Close()
			t.Fatalf("table count = %d", tableCount)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}
}

func TestOpenRecordsLatestMigrationVersion(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	var version int
	var dirty bool
	if err := db.QueryRow("SELECT version, dirty FROM schema_migrations LIMIT 1").Scan(&version, &dirty); err != nil {
		t.Fatalf("read migration version: %v", err)
	}
	if version != 2 || dirty {
		t.Fatalf("migration version = %d, dirty = %t", version, dirty)
	}
	if _, err := db.Exec("SELECT email_verified_at FROM users LIMIT 1"); err != nil {
		t.Fatalf("email verification column missing: %v", err)
	}
}

func TestOpenEnforcesForeignKeys(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO budget_limits(user_id, category, amount_cents)
		VALUES (99, 'groceries', 10000)`)
	if err == nil {
		t.Fatal("foreign key violation should fail")
	}
}

func TestOpenConfiguresSQLite(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d", foreignKeys)
	}

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q", journalMode)
	}

	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d", busyTimeout)
	}
}
