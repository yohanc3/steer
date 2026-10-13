package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"yohanc3/steer/models"
)

// CreateTransactionCategory stores a user-owned option and requeues records that
// were uncategorized only because that user previously had no categories.
func (repository Repository) CreateTransactionCategory(ctx context.Context, userID models.UserID, name, description string) (models.TransactionCategory, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" || len(name) > 64 {
		return models.TransactionCategory{}, fmt.Errorf("category name must contain 1 to 64 characters")
	}
	if len(description) > 500 {
		return models.TransactionCategory{}, fmt.Errorf("category description must contain at most 500 characters")
	}
	category := models.TransactionCategory{ID: uuid.NewString(), UserID: userID, Name: name, Description: description}
	tx, err := repository.Database.BeginTx(ctx, nil)
	if err != nil {
		return models.TransactionCategory{}, fmt.Errorf("begin category creation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO transaction_categories(category_id, user_id, name, description) VALUES (?, ?, ?, ?)`, category.ID, category.UserID, category.Name, category.Description); err != nil {
		return models.TransactionCategory{}, fmt.Errorf("create transaction category: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE transactions SET categorization_status='pending', categorization_source=NULL, categorized_at=NULL, updated_at=unixepoch() WHERE user_id=? AND categorization_status='uncategorized' AND categorization_source='no_categories'`, userID); err != nil {
		return models.TransactionCategory{}, fmt.Errorf("requeue transactions without categories: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return models.TransactionCategory{}, fmt.Errorf("commit category creation: %w", err)
	}
	return category, nil
}

// ListTransactionCategories returns active user-owned choices in deterministic order.
func (repository Repository) ListTransactionCategories(ctx context.Context, userID models.UserID) ([]models.TransactionCategory, error) {
	rows, err := repository.Database.QueryContext(ctx, `SELECT category_id, user_id, name, description FROM transaction_categories WHERE user_id=? AND active=1 ORDER BY name COLLATE NOCASE, category_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list transaction categories: %w", err)
	}
	defer rows.Close()
	var categories []models.TransactionCategory
	for rows.Next() {
		var category models.TransactionCategory
		if err := rows.Scan(&category.ID, &category.UserID, &category.Name, &category.Description); err != nil {
			return nil, fmt.Errorf("scan transaction category: %w", err)
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transaction categories: %w", err)
	}
	return categories, nil
}

// ListPendingCategorizations returns one stable cursor page and any exact-name rule.
func (repository Repository) ListPendingCategorizations(ctx context.Context, afterCreatedAt int64, afterTransactionID string, limit int) ([]models.CategorizationCandidate, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("categorization limit must be greater than zero")
	}
	rows, err := repository.Database.QueryContext(ctx, `
SELECT
    t.transaction_id,
    t.account_id,
    t.amount,
    t.transaction_date,
    t.description,
    t.provider_status,
    t.delivery_status,
    t.transaction_type,
    t.running_balance,
    t.processing_status,
    t.category,
    t.counterparty_name,
    t.counterparty_type,
    t.self_link,
    t.account_link,
    t.user_id,
    remembered.category_id,
    t.created_at
FROM transactions AS t
LEFT JOIN merchant_category_rules AS rule
  ON rule.user_id = t.user_id
 AND rule.normalized_transaction_name = LOWER(TRIM(t.description))
LEFT JOIN transaction_categories AS remembered
  ON remembered.category_id = rule.category_id
 AND remembered.active = 1
WHERE t.provider_status = 'posted'
  AND t.categorization_status = 'pending'
  AND (t.created_at > ? OR (t.created_at = ? AND t.transaction_id > ?))
ORDER BY t.created_at, t.transaction_id
LIMIT ?`, afterCreatedAt, afterCreatedAt, afterTransactionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending categorizations: %w", err)
	}
	defer rows.Close()

	var candidates []models.CategorizationCandidate
	for rows.Next() {
		var candidate models.CategorizationCandidate
		var remembered sql.NullString
		transaction := &candidate.Transaction
		if err := rows.Scan(&transaction.ID, &transaction.AccountID, &transaction.Amount, &transaction.Date, &transaction.Description, &transaction.ProviderStatus, &transaction.DeliveryStatus, &transaction.Type, &transaction.RunningBalance, &transaction.ProcessingStatus, &transaction.Category, &transaction.CounterpartyName, &transaction.CounterpartyType, &transaction.SelfLink, &transaction.AccountLink, &candidate.UserID, &remembered, &candidate.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending categorization: %w", err)
		}
		if remembered.Valid {
			candidate.RememberedCategoryID = &remembered.String
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending categorizations: %w", err)
	}
	return candidates, nil
}

// CompleteCategorization persists a model, rule, or fallback result exactly once.
func (repository Repository) CompleteCategorization(ctx context.Context, result models.CategorizationResult) error {
	if result.Status != models.CategorizationStatusCategorized && result.Status != models.CategorizationStatusUncategorized {
		return fmt.Errorf("categorization result must be categorized or uncategorized")
	}
	if result.Status == models.CategorizationStatusCategorized && result.CategoryID == nil {
		return fmt.Errorf("categorized transaction requires a category")
	}
	if result.Status == models.CategorizationStatusUncategorized && result.CategoryID != nil {
		return fmt.Errorf("uncategorized transaction cannot have a category")
	}
	var probabilities any
	if result.Probabilities != nil {
		encoded, err := json.Marshal(result.Probabilities)
		if err != nil {
			return fmt.Errorf("encode categorization probabilities: %w", err)
		}
		probabilities = string(encoded)
	}
	update, err := repository.Database.ExecContext(ctx, `UPDATE transactions SET user_category_id=?, categorization_status=?, categorization_source=?, categorization_confidence=?, categorization_model=?, categorization_probabilities=?, categorized_at=unixepoch(), updated_at=unixepoch() WHERE transaction_id=? AND categorization_status='pending'`, result.CategoryID, result.Status, result.Source, result.Confidence, result.Model, probabilities, result.TransactionID)
	if err != nil {
		return fmt.Errorf("complete transaction categorization: %w", err)
	}
	changed, err := update.RowsAffected()
	if err != nil {
		return fmt.Errorf("categorized transaction rows: %w", err)
	}
	if changed != 1 {
		return ErrNotFound
	}
	return nil
}

// AssignTransactionCategory applies a user correction to every transaction with
// the same normalized name and remembers it for future transactions.
func (repository Repository) AssignTransactionCategory(ctx context.Context, userID models.UserID, transactionID, categoryID string) error {
	tx, err := repository.Database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user category assignment: %w", err)
	}
	defer tx.Rollback()

	var normalizedName string
	if err := tx.QueryRowContext(ctx, `SELECT LOWER(TRIM(description)) FROM transactions WHERE transaction_id=? AND user_id=?`, transactionID, userID).Scan(&normalizedName); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return fmt.Errorf("get transaction name for category assignment: %w", err)
	}
	var categoryExists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM transaction_categories WHERE category_id=? AND user_id=? AND active=1`, categoryID, userID).Scan(&categoryExists); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return fmt.Errorf("validate transaction category: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO merchant_category_rules(user_id, normalized_transaction_name, category_id) VALUES (?, ?, ?) ON CONFLICT(user_id, normalized_transaction_name) DO UPDATE SET category_id=excluded.category_id, updated_at=unixepoch()`, userID, normalizedName, categoryID); err != nil {
		return fmt.Errorf("remember merchant category: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE transactions SET user_category_id=?, categorization_status='categorized', categorization_source='merchant_rule', categorization_confidence=NULL, categorization_model=NULL, categorization_probabilities=NULL, categorized_at=unixepoch(), updated_at=unixepoch() WHERE user_id=? AND LOWER(TRIM(description))=?`, categoryID, userID, normalizedName); err != nil {
		return fmt.Errorf("apply remembered merchant category: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE transactions SET categorization_source='user', updated_at=unixepoch() WHERE transaction_id=? AND user_id=?`, transactionID, userID)
	if err != nil {
		return fmt.Errorf("mark user category assignment: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user category assignment: %w", err)
	}
	return nil
}
