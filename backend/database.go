package main

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schemaSQL string

// InitializeDatabase bootstraps an empty database. Versioned migrations will
// replace this initializer once the schema begins evolving.
func InitializeDatabase(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}
