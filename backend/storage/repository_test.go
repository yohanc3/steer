package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
	"yohanc3/steer/models"
)

func TestTransactionUpsertAndCursor(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := Repository{DB: db}
	user, err := repository.GetOrCreateUser(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	pending := models.Transaction{ID: "txn-pending", AccountID: "acc", Amount: "-1.00", Date: "2026-08-01", Description: "old", Status: "pending", Type: "card_payment", ProcessingStatus: "pending", SelfLink: "self", AccountLink: "account"}
	if err := repository.UpsertTransactions(context.Background(), user.ID, []models.Transaction{pending}, true); err != nil {
		t.Fatal(err)
	}
	start, err := repository.SyncStartDate(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if start != "2026-08-01" {
		t.Fatalf("start = %q", start)
	}
	complete := pending
	complete.Description = "new"
	complete.ProcessingStatus = "complete"
	complete.Date = "2026-08-03"
	if err := repository.UpsertTransactions(context.Background(), user.ID, []models.Transaction{complete}, false); err != nil {
		t.Fatal(err)
	}
	start, err = repository.SyncStartDate(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if start != "2026-08-03" {
		t.Fatalf("start = %q", start)
	}
}

func TestFinalizeConnectSessionIsAtomicAndSingleUse(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := Repository{DB: db}
	user, err := repository.GetOrCreateUser(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	token := HashToken("token")
	if err := repository.CreateConnectSession(context.Background(), models.ConnectSession{TokenHash: token, UserID: user.ID, Nonce: "nonce", ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	otherUser, err := repository.GetOrCreateUser(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveTellerConnection(context.Background(), otherUser.ID, models.Account{ID: "acc-in-use"}, "enr-existing", "usr-existing", []byte("ciphertext"), []byte("nonce"), "sandbox"); err != nil {
		t.Fatal(err)
	}
	completion := models.ConnectCompletion{TokenHash: token, Account: models.Account{ID: "acc-in-use"}, EnrollmentID: "enr", TellerUserID: "usr", AccessToken: []byte("ciphertext"), AccessTokenNonce: []byte("nonce"), Environment: "sandbox", CompletedAt: time.Now()}
	if err := repository.FinalizeConnectSession(context.Background(), completion); err == nil {
		t.Fatal("finalization with duplicate account succeeded")
	}
	if _, err := repository.GetConnectSession(context.Background(), token, time.Now()); err != nil {
		t.Fatalf("failed finalization consumed session: %v", err)
	}
	completion.Account.ID = "acc-new"
	if err := repository.FinalizeConnectSession(context.Background(), completion); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetConnectSession(context.Background(), token, time.Now()); err == nil {
		t.Fatal("finalized session remained usable")
	}
}
