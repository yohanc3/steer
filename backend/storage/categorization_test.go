package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"yohanc3/steer/models"
)

func TestTransactionCategoriesRequeueOnlyNoCategoryFallbacks(t *testing.T) {
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
	transactions := []models.Transaction{
		{ID: "no-categories", AccountID: "acc", Description: "One", ProviderStatus: "posted"},
		{ID: "uncertain", AccountID: "acc", Description: "Two", ProviderStatus: "posted"},
	}
	if err := repository.UpsertTransactions(context.Background(), user.ID, transactions, false); err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteCategorization(context.Background(), models.CategorizationResult{TransactionID: "no-categories", Status: models.CategorizationStatusUncategorized, Source: models.CategorizationSourceNoCategories}); err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteCategorization(context.Background(), models.CategorizationResult{TransactionID: "uncertain", Status: models.CategorizationStatusUncategorized, Source: models.CategorizationSourceJevUncertain}); err != nil {
		t.Fatal(err)
	}
	category, err := repository.CreateTransactionCategory(context.Background(), user.ID, "Groceries", "Food bought for home")
	if err != nil {
		t.Fatal(err)
	}
	categories, err := repository.ListTransactionCategories(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 1 || categories[0] != category {
		t.Fatalf("categories = %#v", categories)
	}
	candidates, err := repository.ListPendingCategorizations(context.Background(), 0, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Transaction.ID != "no-categories" {
		t.Fatalf("requeued candidates = %#v", candidates)
	}
}

func TestUserCategoryAssignmentUpdatesAndRemembersSameTransactionName(t *testing.T) {
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
	otherUser, err := repository.GetOrCreateUser(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	category, err := repository.CreateTransactionCategory(context.Background(), user.ID, "Coffee", "Coffee shops")
	if err != nil {
		t.Fatal(err)
	}
	transactions := []models.Transaction{
		{ID: "selected", AccountID: "acc", Description: "Coffee Shop", ProviderStatus: "posted"},
		{ID: "same-name", AccountID: "acc", Description: " coffee shop ", ProviderStatus: "posted"},
		{ID: "different", AccountID: "acc", Description: "Bakery", ProviderStatus: "posted"},
	}
	if err := repository.UpsertTransactions(context.Background(), user.ID, transactions, false); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertTransactions(context.Background(), otherUser.ID, []models.Transaction{{ID: "other-user", AccountID: "other", Description: "Coffee Shop", ProviderStatus: "posted"}}, false); err != nil {
		t.Fatal(err)
	}
	if err := repository.AssignTransactionCategory(context.Background(), user.ID, "selected", category.ID); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query(`SELECT transaction_id, user_category_id, categorization_source FROM transactions WHERE transaction_id IN ('selected', 'same-name', 'different', 'other-user') ORDER BY transaction_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	values := map[string][2]string{}
	for rows.Next() {
		var id string
		var categoryID, source *string
		if err := rows.Scan(&id, &categoryID, &source); err != nil {
			t.Fatal(err)
		}
		var value [2]string
		if categoryID != nil {
			value[0] = *categoryID
		}
		if source != nil {
			value[1] = *source
		}
		values[id] = value
	}
	if values["selected"] != [2]string{category.ID, "user"} {
		t.Fatalf("selected assignment = %#v", values["selected"])
	}
	if values["same-name"] != [2]string{category.ID, "merchant_rule"} {
		t.Fatalf("same-name assignment = %#v", values["same-name"])
	}
	if values["different"] != [2]string{} || values["other-user"] != [2]string{} {
		t.Fatalf("unrelated assignments = %#v", values)
	}

	future := models.Transaction{ID: "future", AccountID: "acc", Description: "COFFEE SHOP", ProviderStatus: "posted"}
	if err := repository.UpsertTransactions(context.Background(), user.ID, []models.Transaction{future}, false); err != nil {
		t.Fatal(err)
	}
	candidates, err := repository.ListPendingCategorizations(context.Background(), 0, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	var remembered *string
	for _, candidate := range candidates {
		if candidate.Transaction.ID == "future" {
			remembered = candidate.RememberedCategoryID
		}
	}
	if remembered == nil || *remembered != category.ID {
		t.Fatalf("remembered category = %v", remembered)
	}
}

func TestCompleteCategorizationStoresAuditDataOnce(t *testing.T) {
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
	category, err := repository.CreateTransactionCategory(context.Background(), user.ID, "Fuel", "Gas stations")
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertTransactions(context.Background(), user.ID, []models.Transaction{{ID: "txn", AccountID: "acc", Date: "2026-10-13", Description: "Station", ProviderStatus: "posted"}}, false); err != nil {
		t.Fatal(err)
	}
	confidence := 0.92
	model := "jev-build"
	result := models.CategorizationResult{TransactionID: "txn", CategoryID: &category.ID, Status: models.CategorizationStatusCategorized, Source: models.CategorizationSourceJev, Confidence: &confidence, Model: &model, Probabilities: map[string]float64{category.ID: 0.92, "uncategorized": 0.08}}
	if err := repository.CompleteCategorization(context.Background(), result); err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteCategorization(context.Background(), result); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second completion error = %v", err)
	}
	var categoryID, status, source, storedModel, probabilities string
	var storedConfidence float64
	if err := db.QueryRow(`SELECT user_category_id, categorization_status, categorization_source, categorization_confidence, categorization_model, categorization_probabilities FROM transactions WHERE transaction_id='txn'`).Scan(&categoryID, &status, &source, &storedConfidence, &storedModel, &probabilities); err != nil {
		t.Fatal(err)
	}
	if categoryID != category.ID || status != "categorized" || source != "jev" || storedConfidence != confidence || storedModel != model || probabilities == "" {
		t.Fatalf("stored categorization = %q %q %q %f %q %q", categoryID, status, source, storedConfidence, storedModel, probabilities)
	}
	listed, err := repository.ListTransactions(context.Background(), user.ID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].UserCategoryName == nil || *listed[0].UserCategoryName != "Fuel" || listed[0].CategorizationSource == nil || *listed[0].CategorizationSource != models.CategorizationSourceJev {
		t.Fatalf("listed categorized transaction = %#v", listed)
	}
}
