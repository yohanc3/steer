// Package budget contains the isolated budget-prototype workflow.
package budget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrNoBudget means an action needs a saved budget before it can proceed.
	ErrNoBudget = errors.New("no budget exists yet")
	// ErrNoMapping means no cached decision exists for this source activity.
	ErrNoMapping = errors.New("no category mapping exists")
	// ErrAIUnavailable indicates that budget interpretation cannot reach DeepSeek.
	ErrAIUnavailable = errors.New("budget assistant is unavailable")
)

// Category is one user-defined monthly spending allocation.
type Category struct {
	Name              string `json:"name"`
	MonthlyLimitCents int64  `json:"monthly_limit_cents"`
}

// Budget is the saved flexible budget for one prototype browser session.
type Budget struct {
	MonthlyTotalCents int64      `json:"monthly_total_cents"`
	Categories        []Category `json:"categories"`
	Version           int64      `json:"version"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// Transaction is prototype activity submitted by the review UI.
type Transaction struct {
	ID                   string    `json:"id"`
	Merchant             string    `json:"merchant"`
	ProviderCategory     string    `json:"provider_category"`
	AmountCents          int64     `json:"amount_cents"`
	OccurredAt           time.Time `json:"occurred_at"`
	BudgetVersion        int64     `json:"budget_version"`
	BudgetCategory       *string   `json:"budget_category,omitempty"`
	ClassificationStatus string    `json:"classification_status"`
	ClassificationSource string    `json:"classification_source"`
}

// State is the complete browser-scoped view rendered by the prototype UI.
type State struct {
	Budget       *Budget       `json:"budget"`
	Transactions []Transaction `json:"transactions"`
	Diagnostics  Diagnostics   `json:"diagnostics"`
}

// Diagnostics exposes only safe workflow state needed by the prototype console.
type Diagnostics struct {
	Storage      string `json:"storage"`
	AIModel      string `json:"ai_model"`
	AIConfigured bool   `json:"ai_configured"`
	MappingScope string `json:"mapping_scope"`
}

// ParseResult is the constrained result of translating natural-language budget input.
type ParseResult struct {
	Status            string     `json:"status"`
	Reason            string     `json:"reason,omitempty"`
	MonthlyTotalCents int64      `json:"monthly_total_cents,omitempty"`
	Categories        []Category `json:"categories,omitempty"`
}

// AI limits an external model to budget parsing and category selection.
type AI interface {
	ParseBudget(context.Context, string) (ParseResult, error)
	ClassifyTransaction(context.Context, Transaction, []Category) (string, error)
}

// Store persists prototype data without coupling workflow logic to SQLite.
type Store interface {
	GetPrototypeBudget(context.Context, string) (*Budget, error)
	SavePrototypeBudget(context.Context, string, Budget) error
	DeletePrototypeBudget(context.Context, string) error
	DeletePrototype(context.Context, string) error
	ListPrototypeTransactions(context.Context, string) ([]Transaction, error)
	GetPrototypeMapping(context.Context, string, int64, string) (string, error)
	SavePrototypeMapping(context.Context, string, int64, string, string) error
	CreatePrototypeTransaction(context.Context, string, Transaction) error
	ResolvePrototypeTransaction(context.Context, string, string, string) error
}

// Service coordinates constrained model calls, deterministic validation, and storage.
type Service struct {
	Store        Store
	AI           AI
	AIModel      string
	AIConfigured bool
	Now          func() time.Time
}

// View returns all persisted prototype data for one opaque browser identifier.
func (service Service) View(ctx context.Context, clientID string) (State, error) {
	budget, err := service.Store.GetPrototypeBudget(ctx, clientID)
	if err != nil && !errors.Is(err, ErrNoBudget) {
		return State{}, fmt.Errorf("get budget: %w", err)
	}
	transactions, err := service.Store.ListPrototypeTransactions(ctx, clientID)
	if err != nil {
		return State{}, fmt.Errorf("list transactions: %w", err)
	}
	return State{Budget: budget, Transactions: transactions, Diagnostics: Diagnostics{
		Storage:      "SQLite",
		AIModel:      service.AIModel,
		AIConfigured: service.AIConfigured,
		MappingScope: "browser session + budget version + merchant/category",
	}}, nil
}

// SubmitBudget turns one natural-language request into a fully validated replacement budget.
func (service Service) SubmitBudget(ctx context.Context, clientID, message string) (ParseResult, error) {
	if strings.TrimSpace(message) == "" {
		return ParseResult{}, fmt.Errorf("budget instruction is required")
	}
	previous, err := service.Store.GetPrototypeBudget(ctx, clientID)
	if err != nil && !errors.Is(err, ErrNoBudget) {
		return ParseResult{}, fmt.Errorf("get previous budget: %w", err)
	}
	instruction := message
	if previous != nil {
		current, err := json.Marshal(struct {
			MonthlyTotalCents int64      `json:"monthly_total_cents"`
			Categories        []Category `json:"categories"`
		}{MonthlyTotalCents: previous.MonthlyTotalCents, Categories: previous.Categories})
		if err != nil {
			return ParseResult{}, fmt.Errorf("encode current budget: %w", err)
		}
		instruction = "Current saved budget JSON:\n" + string(current) + "\n\nRequested change:\n" + message
	}
	result, err := service.AI.ParseBudget(ctx, instruction)
	if err != nil {
		return ParseResult{}, err
	}
	if result.Status != "valid" {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "I need a monthly total and complete category allocations."
		}
		return result, nil
	}
	if err := validate(result); err != nil {
		return ParseResult{Status: "invalid", Reason: err.Error()}, nil
	}

	version := int64(1)
	if previous != nil {
		version = previous.Version + 1
	}
	if err := service.Store.SavePrototypeBudget(ctx, clientID, Budget{
		MonthlyTotalCents: result.MonthlyTotalCents,
		Categories:        result.Categories,
		Version:           version,
		UpdatedAt:         service.now(),
	}); err != nil {
		return ParseResult{}, fmt.Errorf("save budget: %w", err)
	}
	return result, nil
}

// DeleteBudget removes the current plan while keeping the prototype session itself.
func (service Service) DeleteBudget(ctx context.Context, clientID string) error {
	if err := service.Store.DeletePrototypeBudget(ctx, clientID); err != nil {
		return fmt.Errorf("delete budget: %w", err)
	}
	return nil
}

// Reset deletes the entire browser-scoped prototype, including its activity and mappings.
func (service Service) Reset(ctx context.Context, clientID string) error {
	if err := service.Store.DeletePrototype(ctx, clientID); err != nil {
		return fmt.Errorf("reset prototype: %w", err)
	}
	return nil
}

// AddTransaction uses a versioned cache before asking the model to map unfamiliar activity.
func (service Service) AddTransaction(ctx context.Context, clientID string, transaction Transaction) (Transaction, error) {
	budget, err := service.Store.GetPrototypeBudget(ctx, clientID)
	if errors.Is(err, ErrNoBudget) {
		return Transaction{}, ErrNoBudget
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("get budget: %w", err)
	}
	if strings.TrimSpace(transaction.Merchant) == "" || strings.TrimSpace(transaction.ProviderCategory) == "" {
		return Transaction{}, fmt.Errorf("merchant and Teller category are required")
	}
	transaction.OccurredAt = service.now()
	transaction.BudgetVersion = budget.Version
	transaction.ClassificationStatus = "unmatched"
	transaction.ClassificationSource = "unmatched"
	signature := sourceSignature(transaction)
	mapping, err := service.Store.GetPrototypeMapping(ctx, clientID, budget.Version, signature)
	if err == nil {
		transaction.BudgetCategory = &mapping
		transaction.ClassificationStatus = "matched"
		transaction.ClassificationSource = "cache"
	} else if !errors.Is(err, ErrNoMapping) {
		return Transaction{}, fmt.Errorf("get category mapping: %w", err)
	} else {
		category, classifyErr := service.AI.ClassifyTransaction(ctx, transaction, budget.Categories)
		if classifyErr != nil {
			return Transaction{}, classifyErr
		}
		if matchingCategory(category, budget.Categories) != "" {
			transaction.BudgetCategory = stringPointer(matchingCategory(category, budget.Categories))
			transaction.ClassificationStatus = "matched"
			transaction.ClassificationSource = "ai"
			if err := service.Store.SavePrototypeMapping(ctx, clientID, budget.Version, signature, *transaction.BudgetCategory); err != nil {
				return Transaction{}, fmt.Errorf("cache category mapping: %w", err)
			}
		}
	}
	if err := service.Store.CreatePrototypeTransaction(ctx, clientID, transaction); err != nil {
		return Transaction{}, fmt.Errorf("save transaction: %w", err)
	}
	return transaction, nil
}

// ResolveUnmatched assigns one unmatched transaction and optionally caches the choice.
func (service Service) ResolveUnmatched(ctx context.Context, clientID, transactionID, category string, remember bool) error {
	budget, err := service.Store.GetPrototypeBudget(ctx, clientID)
	if errors.Is(err, ErrNoBudget) {
		return ErrNoBudget
	}
	if err != nil {
		return fmt.Errorf("get budget: %w", err)
	}
	category = matchingCategory(category, budget.Categories)
	if category == "" {
		return fmt.Errorf("choose one of the current budget categories")
	}
	if err := service.Store.ResolvePrototypeTransaction(ctx, clientID, transactionID, category); err != nil {
		return fmt.Errorf("resolve transaction: %w", err)
	}
	if !remember {
		return nil
	}
	transactions, err := service.Store.ListPrototypeTransactions(ctx, clientID)
	if err != nil {
		return fmt.Errorf("find resolved transaction: %w", err)
	}
	for _, transaction := range transactions {
		if transaction.ID == transactionID {
			if err := service.Store.SavePrototypeMapping(ctx, clientID, budget.Version, sourceSignature(transaction), category); err != nil {
				return fmt.Errorf("cache selected category: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("transaction not found")
}

func (service Service) now() time.Time {
	if service.Now == nil {
		return time.Now().UTC()
	}
	return service.Now().UTC()
}

func validate(result ParseResult) error {
	if result.MonthlyTotalCents <= 0 {
		return fmt.Errorf("monthly budget must be greater than zero")
	}
	if len(result.Categories) == 0 {
		return fmt.Errorf("add at least one budget category")
	}
	seen := make(map[string]struct{}, len(result.Categories))
	var allocated int64
	for _, category := range result.Categories {
		name := strings.TrimSpace(category.Name)
		if name == "" || category.MonthlyLimitCents <= 0 {
			return fmt.Errorf("every category needs a name and a positive amount")
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("each category name must be unique")
		}
		seen[key] = struct{}{}
		allocated += category.MonthlyLimitCents
	}
	if allocated != result.MonthlyTotalCents {
		return fmt.Errorf("category allocations total $%.2f, not the $%.2f monthly budget", float64(allocated)/100, float64(result.MonthlyTotalCents)/100)
	}
	return nil
}

func sourceSignature(transaction Transaction) string {
	return strings.ToLower(strings.TrimSpace(transaction.Merchant)) + "|" + strings.ToLower(strings.TrimSpace(transaction.ProviderCategory))
}

func matchingCategory(candidate string, categories []Category) string {
	for _, category := range categories {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(category.Name)) {
			return category.Name
		}
	}
	return ""
}

func stringPointer(value string) *string { return &value }
