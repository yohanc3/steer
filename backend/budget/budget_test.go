package budget

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSubmitBudgetRejectsIncompletePlanWithoutSaving(t *testing.T) {
	store := &memoryStore{}
	service := Service{Store: store, AI: &fakeAI{parse: ParseResult{Status: "invalid", Reason: "$400 remains unallocated."}}}

	result, err := service.SubmitBudget(context.Background(), "browser", "My budget is $1500 and rent is $1000")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "invalid" || store.budget != nil {
		t.Fatalf("result = %#v, stored budget = %#v", result, store.budget)
	}
}

func TestTransactionClassificationUsesVersionedCache(t *testing.T) {
	store := &memoryStore{}
	ai := &fakeAI{parse: ParseResult{Status: "valid", MonthlyTotalCents: 10000, Categories: []Category{{Name: "Food", MonthlyLimitCents: 10000}}}, category: "Food"}
	service := Service{Store: store, AI: ai, Now: func() time.Time { return time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC) }}
	if _, err := service.SubmitBudget(context.Background(), "browser", "Spend $100 on food"); err != nil {
		t.Fatal(err)
	}

	first, err := service.AddTransaction(context.Background(), "browser", Transaction{ID: "one", Merchant: "Whole Foods", ProviderCategory: "Groceries", AmountCents: 2500})
	if err != nil {
		t.Fatal(err)
	}
	if first.ClassificationStatus != "matched" || first.BudgetCategory == nil || *first.BudgetCategory != "Food" {
		t.Fatalf("first transaction = %#v", first)
	}
	second, err := service.AddTransaction(context.Background(), "browser", Transaction{ID: "two", Merchant: "Whole Foods", ProviderCategory: "Groceries", AmountCents: 3000})
	if err != nil {
		t.Fatal(err)
	}
	if second.ClassificationStatus != "matched" || ai.classifyCalls != 1 {
		t.Fatalf("second transaction = %#v, AI calls = %d", second, ai.classifyCalls)
	}
}

func TestSubmitBudgetSuppliesSavedBudgetForNaturalLanguageEdits(t *testing.T) {
	store := &memoryStore{}
	ai := &fakeAI{parse: ParseResult{Status: "valid", MonthlyTotalCents: 10000, Categories: []Category{{Name: "Other", MonthlyLimitCents: 10000}}}}
	service := Service{Store: store, AI: ai}
	if _, err := service.SubmitBudget(context.Background(), "browser", "Monthly budget 100 USD: 100 other."); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SubmitBudget(context.Background(), "browser", "Move 20 USD from other to car fixes."); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ai.lastInstruction, "Current saved budget JSON") || !strings.Contains(ai.lastInstruction, "Requested change") {
		t.Fatalf("edit instruction = %q", ai.lastInstruction)
	}
}

func TestActionPlanUsesModelDependencyIndexes(t *testing.T) {
	var plan ActionPlan
	if err := json.Unmarshal([]byte(`{"actions":[{"type":"show_budget","depends_on_indexes":[0],"arguments":{}}]}`), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Actions[0].DependsOn[0] != 0 {
		t.Fatalf("plan = %#v", plan)
	}
}

type fakeAI struct {
	parse           ParseResult
	category        string
	classifyCalls   int
	lastInstruction string
}

func (ai *fakeAI) ParseBudget(_ context.Context, instruction string) (ParseResult, error) {
	ai.lastInstruction = instruction
	return ai.parse, nil
}

func (ai *fakeAI) ClassifyTransaction(_ context.Context, _ Transaction, _ []Category) (string, error) {
	ai.classifyCalls++
	return ai.category, nil
}

type memoryStore struct {
	budget       *Budget
	transactions []Transaction
	mappings     map[string]string
}

func (store *memoryStore) GetPrototypeBudget(_ context.Context, _ string) (*Budget, error) {
	if store.budget == nil {
		return nil, ErrNoBudget
	}
	copy := *store.budget
	copy.Categories = append([]Category(nil), store.budget.Categories...)
	return &copy, nil
}

func (store *memoryStore) SavePrototypeBudget(_ context.Context, _ string, value Budget) error {
	store.budget = &value
	return nil
}

func (store *memoryStore) DeletePrototypeBudget(_ context.Context, _ string) error {
	store.budget = nil
	return nil
}
func (store *memoryStore) DeletePrototype(_ context.Context, _ string) error {
	store.budget, store.transactions, store.mappings = nil, nil, nil
	return nil
}
func (store *memoryStore) ListPrototypeTransactions(_ context.Context, _ string) ([]Transaction, error) {
	return append([]Transaction(nil), store.transactions...), nil
}
func (store *memoryStore) GetPrototypeMapping(_ context.Context, _ string, version int64, signature string) (string, error) {
	category, found := store.mappings[mappingKey(version, signature)]
	if !found {
		return "", ErrNoMapping
	}
	return category, nil
}
func (store *memoryStore) SavePrototypeMapping(_ context.Context, _ string, version int64, signature, category string) error {
	if store.mappings == nil {
		store.mappings = map[string]string{}
	}
	store.mappings[mappingKey(version, signature)] = category
	return nil
}
func (store *memoryStore) CreatePrototypeTransaction(_ context.Context, _ string, transaction Transaction) error {
	store.transactions = append(store.transactions, transaction)
	return nil
}
func (store *memoryStore) ResolvePrototypeTransaction(_ context.Context, _ string, id, category string) error {
	for index := range store.transactions {
		if store.transactions[index].ID == id {
			store.transactions[index].BudgetCategory = stringPointer(category)
			store.transactions[index].ClassificationStatus = "matched"
			return nil
		}
	}
	return ErrNoBudget
}

func mappingKey(version int64, signature string) string {
	return strconv.FormatInt(version, 10) + ":" + signature
}
