package storage

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"yohanc3/steer/budget"
	"yohanc3/steer/models"
)

func TestTelegramAgentCreatesBudget(t *testing.T) {
	agent, repository, user := newTelegramBudgetAgent(t)

	receipts, err := agent.Handle(context.Background(), user.ID, 101, "create a $1500 monthly budget")
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || receipts[0].Status != "applied" {
		t.Fatalf("receipts = %#v", receipts)
	}
	stored, err := repository.GetTelegramBudget(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.MonthlyTotalCents != 150000 || stored.Version != 1 || len(stored.Categories) != 2 {
		t.Fatalf("budget = %#v", stored)
	}
}

func TestTelegramAgentClarifiesUnfundedCategoryDeletion(t *testing.T) {
	agent, repository, user := newTelegramBudgetAgent(t)
	if _, err := agent.Handle(context.Background(), user.ID, 101, "create a $1500 monthly budget"); err != nil {
		t.Fatal(err)
	}

	receipts, err := agent.Handle(context.Background(), user.ID, 102, "delete my food category")
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || receipts[0].Status != "clarify" {
		t.Fatalf("receipts = %#v", receipts)
	}
	stored, err := repository.GetTelegramBudget(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != 1 || len(stored.Categories) != 2 {
		t.Fatalf("budget changed after clarification: %#v", stored)
	}
}

func TestTelegramAgentAppliesIndependentMultipleActions(t *testing.T) {
	agent, repository, user := newTelegramBudgetAgent(t)
	if _, err := agent.Handle(context.Background(), user.ID, 101, "create a $1500 monthly budget"); err != nil {
		t.Fatal(err)
	}

	receipts, err := agent.Handle(context.Background(), user.ID, 103, "move money and show my budget")
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 2 || receipts[0].Status != "applied" || receipts[1].Status != "applied" {
		t.Fatalf("receipts = %#v", receipts)
	}
	stored, err := repository.GetTelegramBudget(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != 2 || categoryAmount(stored.Categories, "Food") != 10000 || categoryAmount(stored.Categories, "Car fixes") != 10000 {
		t.Fatalf("budget = %#v", stored)
	}
}

func TestTelegramAgentCreatesBudgetDespiteConversationalFiller(t *testing.T) {
	agent, repository, user := newTelegramBudgetAgent(t)

	receipts, err := agent.Handle(context.Background(), user.ID, 201, "Hey Steer, I finally want to get serious about this. Please make my monthly budget 1500 total: 1300 for rent and 200 for food. Thanks!")
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || receipts[0].Status != "applied" {
		t.Fatalf("receipts = %#v", receipts)
	}
	stored, err := repository.GetTelegramBudget(context.Background(), user.ID)
	if err != nil || stored.MonthlyTotalCents != 150000 {
		t.Fatalf("budget = %#v, error = %v", stored, err)
	}
}

func TestTelegramAgentClarifiesFillerAroundUnfundedDeletion(t *testing.T) {
	agent, _, user := newTelegramBudgetAgent(t)
	if _, err := agent.Handle(context.Background(), user.ID, 201, "create a $1500 monthly budget"); err != nil {
		t.Fatal(err)
	}

	receipts, err := agent.Handle(context.Background(), user.ID, 202, "Food has not really worked for me lately, so whenever you get a chance please remove it from my budget. I am not sure what should happen to that money.")
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || receipts[0].Status != "clarify" {
		t.Fatalf("receipts = %#v", receipts)
	}
}

func TestTelegramAgentExecutesMultipleActionsDespiteFiller(t *testing.T) {
	agent, repository, user := newTelegramBudgetAgent(t)
	if _, err := agent.Handle(context.Background(), user.ID, 201, "create a $1500 monthly budget"); err != nil {
		t.Fatal(err)
	}

	receipts, err := agent.Handle(context.Background(), user.ID, 203, "Could you please move 100 dollars from Food to a new Car fixes category? Also, after that, show me where everything stands so I can double-check it.")
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 2 || receipts[0].Status != "applied" || receipts[1].Status != "applied" {
		t.Fatalf("receipts = %#v", receipts)
	}
	stored, err := repository.GetTelegramBudget(context.Background(), user.ID)
	if err != nil || categoryAmount(stored.Categories, "Car fixes") != 10000 {
		t.Fatalf("budget = %#v, error = %v", stored, err)
	}
}

func TestTelegramAgentClarifiesWhenPlannerReturnsNoJSON(t *testing.T) {
	agent, repository, user := newTelegramBudgetAgent(t)
	agent.Planner = unavailablePlanner{}

	receipts, err := agent.Handle(context.Background(), user.ID, 204, "split my budget in half")
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || receipts[0].Status != "clarify" {
		t.Fatalf("receipts = %#v", receipts)
	}
	var result budget.ActionResult
	if err := json.Unmarshal(receipts[0].Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Summary != "Do you mean split your entire monthly budget, or split the current Other allocation? Please include the amount or category to split." {
		t.Fatalf("summary = %q", result.Summary)
	}
	if _, err := repository.GetTelegramBudget(context.Background(), user.ID); !errors.Is(err, budget.ErrNoBudget) {
		t.Fatalf("planner failure changed budget: %v", err)
	}
}

func newTelegramBudgetAgent(t *testing.T) (budget.TelegramAgent, Repository, models.User) {
	t.Helper()
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "steer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := Repository{Database: database}
	user, err := repository.GetOrCreateUser(context.Background(), 9876)
	if err != nil {
		t.Fatal(err)
	}
	return budget.TelegramAgent{Store: repository, Planner: budgetTestPlanner{}, Now: func() time.Time { return time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC) }}, repository, user
}

type budgetTestPlanner struct{}

type unavailablePlanner struct{}

func (unavailablePlanner) PlanTelegramBudgetActions(context.Context, string, *budget.Budget) (budget.ActionPlan, error) {
	return budget.ActionPlan{}, budget.ErrAIUnavailable
}

func (budgetTestPlanner) PlanTelegramBudgetActions(_ context.Context, message string, _ *budget.Budget) (budget.ActionPlan, error) {
	switch message {
	case "Hey Steer, I finally want to get serious about this. Please make my monthly budget 1500 total: 1300 for rent and 200 for food. Thanks!":
		fallthrough
	case "create a $1500 monthly budget":
		return budget.ActionPlan{Actions: []budget.PlannedAction{{Type: "create_budget", Arguments: actionArguments(map[string]any{"monthly_total_cents": 150000, "categories": []map[string]any{{"name": "Rent", "monthly_limit_cents": 130000}, {"name": "Food", "monthly_limit_cents": 20000}}})}}}, nil
	case "delete my food category", "Food has not really worked for me lately, so whenever you get a chance please remove it from my budget. I am not sure what should happen to that money.":
		return budget.ActionPlan{Actions: []budget.PlannedAction{{Type: "clarify", Explanation: "Where should the Food allocation be moved?", Arguments: json.RawMessage(`{}`)}}}, nil
	case "move money and show my budget", "Could you please move 100 dollars from Food to a new Car fixes category? Also, after that, show me where everything stands so I can double-check it.":
		return budget.ActionPlan{Actions: []budget.PlannedAction{{Type: "move_allocation", Arguments: actionArguments(map[string]any{"from": "Food", "to": "Car fixes", "cents": 10000})}, {Type: "show_budget", Arguments: json.RawMessage(`{}`)}}}, nil
	default:
		return budget.ActionPlan{}, nil
	}
}

func actionArguments(value any) json.RawMessage { encoded, _ := json.Marshal(value); return encoded }

func categoryAmount(categories []budget.Category, name string) int64 {
	for _, category := range categories {
		if category.Name == name {
			return category.MonthlyLimitCents
		}
	}
	return 0
}
