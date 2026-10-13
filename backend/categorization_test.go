package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"yohanc3/steer/models"
)

type categorizationStoreStub struct {
	mu         sync.Mutex
	candidates []models.CategorizationCandidate
	categories map[models.UserID][]models.TransactionCategory
	results    []models.CategorizationResult
}

func (stub *categorizationStoreStub) ListPendingCategorizations(_ context.Context, afterCreatedAt int64, afterTransactionID string, limit int) ([]models.CategorizationCandidate, error) {
	var result []models.CategorizationCandidate
	for _, candidate := range stub.candidates {
		if candidate.CreatedAt < afterCreatedAt || (candidate.CreatedAt == afterCreatedAt && candidate.Transaction.ID <= afterTransactionID) {
			continue
		}
		result = append(result, candidate)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func (stub *categorizationStoreStub) ListTransactionCategories(_ context.Context, userID models.UserID) ([]models.TransactionCategory, error) {
	return stub.categories[userID], nil
}

func (stub *categorizationStoreStub) CompleteCategorization(_ context.Context, result models.CategorizationResult) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.results = append(stub.results, result)
	return nil
}

type categorizerStub struct {
	mu        sync.Mutex
	decisions map[string]models.CategoryDecision
	errors    map[string]error
	calls     []string
}

func (stub *categorizerStub) Categorize(_ context.Context, transaction models.Transaction, _ []models.TransactionCategory) (models.CategoryDecision, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.calls = append(stub.calls, transaction.ID)
	return stub.decisions[transaction.ID], stub.errors[transaction.ID]
}

func TestCategorizationWorkerUsesRulesAndLeavesUncertainTransactionsUncategorized(t *testing.T) {
	remembered := "groceries"
	store := &categorizationStoreStub{
		candidates: []models.CategorizationCandidate{
			{Transaction: models.Transaction{ID: "confident"}, UserID: "user", CreatedAt: 1},
			{Transaction: models.Transaction{ID: "error"}, UserID: "user", CreatedAt: 1},
			{Transaction: models.Transaction{ID: "no-categories"}, UserID: "empty", CreatedAt: 1},
			{Transaction: models.Transaction{ID: "rule"}, UserID: "user", RememberedCategoryID: &remembered, CreatedAt: 1},
			{Transaction: models.Transaction{ID: "uncertain"}, UserID: "user", CreatedAt: 1},
		},
		categories: map[models.UserID][]models.TransactionCategory{"user": {{ID: "groceries", Name: "Groceries"}}},
	}
	categorizer := &categorizerStub{
		decisions: map[string]models.CategoryDecision{
			"confident": {CategoryID: &remembered, Confidence: 0.9, Model: "jev", Probabilities: map[string]float64{"groceries": 0.9}},
			"uncertain": {CategoryID: &remembered, Confidence: 0.4, Model: "jev", Probabilities: map[string]float64{"groceries": 0.6}},
		},
		errors: map[string]error{"error": errors.New("unavailable")},
	}
	worker, err := NewTransactionCategorizationWorker(store, categorizer, time.Minute, 3, 0.75)
	if err != nil {
		t.Fatal(err)
	}
	worker.categorizeCycle(context.Background())

	store.mu.Lock()
	results := append([]models.CategorizationResult(nil), store.results...)
	store.mu.Unlock()
	if len(results) != 5 {
		t.Fatalf("results = %#v", results)
	}
	byID := make(map[string]models.CategorizationResult, len(results))
	for _, result := range results {
		byID[result.TransactionID] = result
	}
	if byID["rule"].Source != models.CategorizationSourceMerchantRule || byID["rule"].CategoryID == nil {
		t.Fatalf("rule result = %#v", byID["rule"])
	}
	if byID["confident"].Status != models.CategorizationStatusCategorized || byID["confident"].Source != models.CategorizationSourceJev {
		t.Fatalf("confident result = %#v", byID["confident"])
	}
	if byID["uncertain"].Status != models.CategorizationStatusUncategorized || byID["uncertain"].Source != models.CategorizationSourceJevUncertain || byID["uncertain"].CategoryID != nil {
		t.Fatalf("uncertain result = %#v", byID["uncertain"])
	}
	if byID["error"].Source != models.CategorizationSourceJevError || byID["error"].Status != models.CategorizationStatusUncategorized {
		t.Fatalf("error result = %#v", byID["error"])
	}
	if byID["no-categories"].Source != models.CategorizationSourceNoCategories {
		t.Fatalf("no-category result = %#v", byID["no-categories"])
	}
	categorizer.mu.Lock()
	defer categorizer.mu.Unlock()
	if len(categorizer.calls) != 3 {
		t.Fatalf("Jev calls = %#v", categorizer.calls)
	}
}

func TestNewTransactionCategorizationWorkerValidatesPolicy(t *testing.T) {
	store := &categorizationStoreStub{}
	categorizer := &categorizerStub{}
	if _, err := NewTransactionCategorizationWorker(store, categorizer, 0, 1, 0.5); err == nil {
		t.Fatal("zero interval was accepted")
	}
	if _, err := NewTransactionCategorizationWorker(store, categorizer, time.Minute, 0, 0.5); err == nil {
		t.Fatal("zero concurrency was accepted")
	}
	if _, err := NewTransactionCategorizationWorker(store, categorizer, time.Minute, 1, 1.1); err == nil {
		t.Fatal("invalid confidence threshold was accepted")
	}
}
