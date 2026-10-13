package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yohanc3/steer/models"
)

type transactionCategorizationStore interface {
	ListPendingCategorizations(context.Context, int64, string, int) ([]models.CategorizationCandidate, error)
	ListTransactionCategories(context.Context, models.UserID) ([]models.TransactionCategory, error)
	CompleteCategorization(context.Context, models.CategorizationResult) error
}

type transactionCategorizer interface {
	Categorize(context.Context, models.Transaction, []models.TransactionCategory) (models.CategoryDecision, error)
}

// TransactionCategorizationWorker classifies pending transactions in bounded batches.
type TransactionCategorizationWorker struct {
	store               transactionCategorizationStore
	categorizer         transactionCategorizer
	interval            time.Duration
	concurrency         int
	confidenceThreshold float64
}

func NewTransactionCategorizationWorker(store transactionCategorizationStore, categorizer transactionCategorizer, interval time.Duration, concurrency int, confidenceThreshold float64) (*TransactionCategorizationWorker, error) {
	if store == nil {
		return nil, fmt.Errorf("transaction categorization store is required")
	}
	if categorizer == nil {
		return nil, fmt.Errorf("transaction categorizer is required")
	}
	if interval <= 0 {
		return nil, fmt.Errorf("categorization interval must be greater than zero")
	}
	if concurrency <= 0 {
		return nil, fmt.Errorf("categorization concurrency must be greater than zero")
	}
	if confidenceThreshold < 0 || confidenceThreshold > 1 {
		return nil, fmt.Errorf("categorization confidence threshold must be between zero and one")
	}
	return &TransactionCategorizationWorker{store: store, categorizer: categorizer, interval: interval, concurrency: concurrency, confidenceThreshold: confidenceThreshold}, nil
}

// Run categorizes existing work immediately and then drains new work on a timer.
func (worker *TransactionCategorizationWorker) Run(ctx context.Context) {
	worker.categorizeCycle(ctx)
	ticker := time.NewTicker(worker.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			worker.categorizeCycle(ctx)
		}
	}
}

func (worker *TransactionCategorizationWorker) categorizeCycle(ctx context.Context) {
	var afterCreatedAt int64
	var afterTransactionID string
	for ctx.Err() == nil {
		candidates, err := worker.store.ListPendingCategorizations(ctx, afterCreatedAt, afterTransactionID, worker.concurrency)
		if err != nil {
			slog.Log(ctx, slog.LevelError, "list pending transaction categorizations", "error", err)
			return
		}
		if len(candidates) == 0 {
			return
		}

		worker.categorizeBatch(ctx, candidates)
		last := candidates[len(candidates)-1]
		afterCreatedAt = last.CreatedAt
		afterTransactionID = last.Transaction.ID
		if len(candidates) < worker.concurrency {
			return
		}
	}
}

func (worker *TransactionCategorizationWorker) categorizeBatch(ctx context.Context, candidates []models.CategorizationCandidate) {
	var wait sync.WaitGroup
	for _, candidate := range candidates {
		candidate := candidate
		wait.Add(1)
		go func() {
			defer wait.Done()
			worker.categorizeOne(ctx, candidate)
		}()
	}
	wait.Wait()
}

func (worker *TransactionCategorizationWorker) categorizeOne(ctx context.Context, candidate models.CategorizationCandidate) {
	if candidate.RememberedCategoryID != nil {
		worker.complete(ctx, models.CategorizationResult{
			TransactionID: candidate.Transaction.ID,
			CategoryID:    candidate.RememberedCategoryID,
			Status:        models.CategorizationStatusCategorized,
			Source:        models.CategorizationSourceMerchantRule,
		})
		return
	}

	categories, err := worker.store.ListTransactionCategories(ctx, candidate.UserID)
	if err != nil {
		slog.Log(ctx, slog.LevelError, "list user transaction categories", "transaction_id", candidate.Transaction.ID, "error", err)
		return
	}
	if len(categories) == 0 {
		worker.complete(ctx, models.CategorizationResult{TransactionID: candidate.Transaction.ID, Status: models.CategorizationStatusUncategorized, Source: models.CategorizationSourceNoCategories})
		return
	}

	decision, err := worker.categorizer.Categorize(ctx, candidate.Transaction, categories)
	if err != nil {
		slog.Log(ctx, slog.LevelError, "categorize transaction with Jev", "transaction_id", candidate.Transaction.ID, "error", err)
		worker.complete(ctx, models.CategorizationResult{TransactionID: candidate.Transaction.ID, Status: models.CategorizationStatusUncategorized, Source: models.CategorizationSourceJevError})
		return
	}
	confidence := decision.Confidence
	model := decision.Model
	result := models.CategorizationResult{
		TransactionID: candidate.Transaction.ID,
		CategoryID:    decision.CategoryID,
		Status:        models.CategorizationStatusCategorized,
		Source:        models.CategorizationSourceJev,
		Confidence:    &confidence,
		Model:         &model,
		Probabilities: decision.Probabilities,
	}
	if decision.CategoryID == nil || decision.Confidence < worker.confidenceThreshold || !containsCategory(categories, *decision.CategoryID) {
		result.CategoryID = nil
		result.Status = models.CategorizationStatusUncategorized
		result.Source = models.CategorizationSourceJevUncertain
	}
	worker.complete(ctx, result)
}

func (worker *TransactionCategorizationWorker) complete(ctx context.Context, result models.CategorizationResult) {
	if err := worker.store.CompleteCategorization(ctx, result); err != nil {
		slog.Log(ctx, slog.LevelError, "complete transaction categorization", "transaction_id", result.TransactionID, "error", err)
	}
}

func containsCategory(categories []models.TransactionCategory, categoryID string) bool {
	for _, category := range categories {
		if category.ID == categoryID {
			return true
		}
	}
	return false
}
