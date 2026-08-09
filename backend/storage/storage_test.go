package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenAppliesMigrationsIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "steer.sqlite")
	for range 2 {
		db, err := Open(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('users','teller_connect_sessions','transactions')`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 3 {
			t.Fatalf("table count = %d", count)
		}
		_ = db.Close()
	}
}

func TestOpenMigratesLegacyDatabaseToTellerPollingSchema(t *testing.T) {
	databaseURL := filepath.Join(t.TempDir(), "steer.sqlite")
	legacy, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"000001_initial.up.sql", "000002_email_verification.up.sql"} {
		sqlText, err := os.ReadFile(filepath.Join("migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := legacy.Exec(string(sqlText)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	if _, err := legacy.Exec(`
		INSERT INTO users(telegram_user_id, telegram_chat_id, name) VALUES (7, 9, 'Ada');
		INSERT INTO enrollments(enrollment_id, user_id, teller_user_id, access_token_ciphertext, access_token_nonce, environment)
		VALUES ('enr', 7, 'usr', X'01', X'02', 'sandbox');
		INSERT INTO accounts(account_id, enrollment_id, institution_id, institution_name, name, type, subtype, currency, last_four, status)
		VALUES ('acc', 'enr', 'inst', 'Bank', 'Checking', 'depository', 'checking', 'USD', '1234', 'open');
		CREATE TABLE schema_migrations (version INTEGER NOT NULL PRIMARY KEY, dirty BOOLEAN NOT NULL);
		INSERT INTO schema_migrations(version, dirty) VALUES (2, FALSE);
	`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := Repository{Database: db}
	user, err := repository.GetUser(context.Background(), "7")
	if err != nil {
		t.Fatal(err)
	}
	if user.TelegramConversationID != 9 || user.TellerAccountID != "acc" || user.TellerUserID != "usr" {
		t.Fatalf("migrated user = %#v", user)
	}
}

func TestBackupWritesSQLiteSnapshot(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "steer.sqlite")
	database, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	backupPath, err := Backup(context.Background(), database, t.TempDir(), time.Date(2026, 8, 9, 1, 2, 3, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(backupPath) != "steer-20260809T010203Z.sqlite" {
		t.Fatalf("backup path = %s", backupPath)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatal(err)
	}
}
