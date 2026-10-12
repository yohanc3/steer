package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
	"yohanc3/steer/models"
)

func TestTransactionUpsertUpdatesProviderData(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := Repository{Database: db}
	user, err := repository.GetOrCreateUser(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	pending := models.Transaction{ID: "txn-pending", AccountID: "acc", Amount: "-1.00", Date: "2026-08-01", Description: "old", ProviderStatus: "pending", Type: "card_payment", ProcessingStatus: "pending", SelfLink: "self", AccountLink: "account"}
	if err := repository.UpsertTransactions(context.Background(), user.ID, []models.Transaction{pending}, true); err != nil {
		t.Fatal(err)
	}
	complete := pending
	complete.Description = "new"
	complete.ProcessingStatus = "complete"
	complete.Date = "2026-08-03"
	if err := repository.UpsertTransactions(context.Background(), user.ID, []models.Transaction{complete}, false); err != nil {
		t.Fatal(err)
	}
	listed, err := repository.ListTransactions(context.Background(), user.ID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Date != "2026-08-03" || listed[0].Description != "new" {
		t.Fatalf("transactions = %#v", listed)
	}
	var baseline int
	if err := db.QueryRow(`SELECT baseline FROM transactions WHERE transaction_id='txn-pending'`).Scan(&baseline); err != nil {
		t.Fatal(err)
	}
	if baseline != 1 {
		t.Fatalf("baseline was reset by overlap upsert: %d", baseline)
	}
}

func TestListTransactionsFiltersUserAndDate(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := Repository{Database: db}
	user, err := repository.GetOrCreateUser(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	transactions := []models.Transaction{{ID: "old", AccountID: "acc", Amount: "-1.00", Date: "2026-08-01", Description: "old"}, {ID: "recent", AccountID: "acc", Amount: "-2.00", Date: "2026-08-08", Description: "recent"}}
	if err := repository.UpsertTransactions(context.Background(), user.ID, transactions, false); err != nil {
		t.Fatal(err)
	}
	listed, err := repository.ListTransactions(context.Background(), user.ID, time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != "recent" {
		t.Fatalf("transactions = %#v", listed)
	}
	if listed[0].DeliveryStatus != models.DeliveryStatusSettled {
		t.Fatalf("delivery status = %q", listed[0].DeliveryStatus)
	}
}

func TestTransactionUpsertPreservesDeliveredStatus(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := Repository{Database: db}
	user, err := repository.GetOrCreateUser(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	transaction := models.Transaction{ID: "txn", AccountID: "acc", Amount: "-1.00", Date: "2026-10-11", Description: "Coffee", ProviderStatus: "posted"}
	if err := repository.UpsertTransactions(context.Background(), user.ID, []models.Transaction{transaction}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE transactions SET delivery_status='delivered' WHERE transaction_id='txn'`); err != nil {
		t.Fatal(err)
	}
	transaction.Description = "Updated Coffee"
	if err := repository.UpsertTransactions(context.Background(), user.ID, []models.Transaction{transaction}, false); err != nil {
		t.Fatal(err)
	}
	listed, err := repository.ListTransactions(context.Background(), user.ID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].DeliveryStatus != models.DeliveryStatusDelivered {
		t.Fatalf("transactions = %#v", listed)
	}
	if _, err := db.Exec(`UPDATE transactions SET delivery_status='unknown' WHERE transaction_id='txn'`); err == nil {
		t.Fatal("invalid delivery status was accepted")
	}
}

func TestSettledTransactionDeliveriesAreFilteredBatchedAndMarked(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := Repository{Database: db}
	user, err := repository.GetOrCreateUser(context.Background(), 987)
	if err != nil {
		t.Fatal(err)
	}
	posted := func(id string) models.Transaction {
		return models.Transaction{ID: id, AccountID: "acc", Amount: "-1.00", Date: "2026-10-11", Description: id, ProviderStatus: "posted"}
	}
	if err := repository.UpsertTransactions(context.Background(), user.ID, []models.Transaction{posted("b"), posted("a"), posted("delivered")}, false); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertTransactions(context.Background(), user.ID, []models.Transaction{posted("baseline")}, true); err != nil {
		t.Fatal(err)
	}
	pending := posted("pending")
	pending.ProviderStatus = "pending"
	if err := repository.UpsertTransactions(context.Background(), user.ID, []models.Transaction{pending}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE transactions SET delivery_status='delivered' WHERE transaction_id='delivered'`); err != nil {
		t.Fatal(err)
	}

	first, err := repository.ListSettledTransactionDeliveries(context.Background(), 0, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Transaction.ID != "a" || first[0].TelegramConversationID != 987 {
		t.Fatalf("first batch = %#v", first)
	}
	second, err := repository.ListSettledTransactionDeliveries(context.Background(), first[0].CreatedAt, first[0].Transaction.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Transaction.ID != "b" {
		t.Fatalf("second batch = %#v", second)
	}
	if err := repository.MarkTransactionDelivered(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkTransactionDelivered(context.Background(), "a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delivery mark error = %v", err)
	}
	remaining, err := repository.ListSettledTransactionDeliveries(context.Background(), 0, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].Transaction.ID != "b" {
		t.Fatalf("remaining deliveries = %#v", remaining)
	}
}

func TestListConnectedUsersReleasesRowsBeforeLoadingUsers(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := Repository{Database: db}
	user, err := repository.GetOrCreateUser(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveTellerConnection(context.Background(), user.ID, models.Account{ID: "acc"}, "usr", []byte("ciphertext"), []byte("nonce"), "sandbox"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	users, err := repository.ListConnectedUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].ID != user.ID {
		t.Fatalf("users = %#v", users)
	}
}

func TestFinalizeConnectSessionIsAtomicAndSingleUse(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := Repository{Database: db}
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
	if err := repository.SaveTellerConnection(context.Background(), otherUser.ID, models.Account{ID: "acc-in-use"}, "usr-existing", []byte("ciphertext"), []byte("nonce"), "sandbox"); err != nil {
		t.Fatal(err)
	}
	completion := models.ConnectCompletion{TokenHash: token, TellerAccount: models.Account{ID: "acc-in-use"}, TellerUserID: "usr", EncryptedAccessToken: []byte("ciphertext"), AccessTokenNonce: []byte("nonce"), TellerEnvironment: "sandbox", CompletedAt: time.Now()}
	if err := repository.FinalizeConnectSession(context.Background(), completion); err == nil {
		t.Fatal("finalization with duplicate account succeeded")
	}
	if _, err := repository.GetConnectSession(context.Background(), token, time.Now()); err != nil {
		t.Fatalf("failed finalization consumed session: %v", err)
	}
	completion.TellerAccount.ID = "acc-new"
	if err := repository.FinalizeConnectSession(context.Background(), completion); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetConnectSession(context.Background(), token, time.Now()); err == nil {
		t.Fatal("finalized session remained usable")
	}
}
