package main

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestInitializeDatabaseCreatesRequiredTables(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := InitializeDatabase(context.Background(), db); err != nil {
		t.Fatalf("initialize database: %v", err)
	}

	for _, table := range []string{"user", "otp"} {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("expected table %q: %v", table, err)
		}
	}
}
