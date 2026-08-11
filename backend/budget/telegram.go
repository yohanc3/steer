package budget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"yohanc3/steer/models"
)

// TelegramStore persists real user budgets and idempotent agent action receipts.
type TelegramStore interface {
	GetTelegramBudget(context.Context, models.UserID) (*Budget, error)
	SaveTelegramBudget(context.Context, models.UserID, Budget) error
	SaveTelegramBudgetAction(context.Context, TelegramActionReceipt) (TelegramActionReceipt, error)
	ListTelegramBudgetActions(context.Context, models.UserID, int64) ([]TelegramActionReceipt, error)
}

// PlannedAction is a model proposal; it cannot mutate data by itself.
type PlannedAction struct {
	Type        string          `json:"type"`
	DependsOn   []int           `json:"depends_on_indexes,omitempty"`
	Arguments   json.RawMessage `json:"arguments"`
	Explanation string          `json:"explanation,omitempty"`
}

// ActionPlan is the complete, structured result of interpreting one Telegram message.
type ActionPlan struct {
	Actions []PlannedAction `json:"actions"`
}

// TelegramActionReceipt is a durable result for one independently handled action.
type TelegramActionReceipt struct {
	ID                string          `json:"id"`
	UserID            models.UserID   `json:"-"`
	TelegramMessageID int64           `json:"telegram_message_id"`
	ActionIndex       int             `json:"action_index"`
	ActionType        string          `json:"action_type"`
	Payload           json.RawMessage `json:"payload"`
	Status            string          `json:"status"`
	Result            json.RawMessage `json:"result"`
}

// ActionResult is the stable user-facing outcome stored for every action.
type ActionResult struct {
	Summary string  `json:"summary"`
	Budget  *Budget `json:"budget,omitempty"`
}

// TelegramPlanner converts a request plus current state into a constrained action plan.
type TelegramPlanner interface {
	PlanTelegramBudgetActions(context.Context, string, *Budget) (ActionPlan, error)
}

// TelegramAgent executes independent action proposals for an existing Telegram user.
type TelegramAgent struct {
	Store   TelegramStore
	Planner TelegramPlanner
	Now     func() time.Time
}

// Handle plans and executes all independent actions from one Telegram message.
func (agent TelegramAgent) Handle(ctx context.Context, userID models.UserID, messageID int64, message string) ([]TelegramActionReceipt, error) {
	if strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("describe the budget change after /budget")
	}
	if prior, err := agent.Store.ListTelegramBudgetActions(ctx, userID, messageID); err == nil && len(prior) > 0 {
		return prior, nil
	} else if err != nil {
		return nil, fmt.Errorf("find prior actions: %w", err)
	}
	current, err := agent.Store.GetTelegramBudget(ctx, userID)
	if err != nil && !errors.Is(err, ErrNoBudget) {
		return nil, fmt.Errorf("get current budget: %w", err)
	}
	plan, err := agent.Planner.PlanTelegramBudgetActions(ctx, message, current)
	if err != nil {
		if errors.Is(err, ErrAIUnavailable) {
			receipt := TelegramActionReceipt{
				ID:                fmt.Sprintf("%s:%d:0", userID, messageID),
				UserID:            userID,
				TelegramMessageID: messageID,
				ActionIndex:       0,
				ActionType:        "clarify",
				Payload:           json.RawMessage(`{}`),
				Status:            "clarify",
				Result:            resultJSON(clarificationFor(message), nil),
			}
			stored, storeErr := agent.Store.SaveTelegramBudgetAction(ctx, receipt)
			if storeErr != nil {
				return nil, fmt.Errorf("save unavailable-model clarification: %w", storeErr)
			}
			return []TelegramActionReceipt{stored}, nil
		}
		return nil, err
	}
	if len(plan.Actions) == 0 {
		return nil, fmt.Errorf("budget assistant did not propose an action")
	}
	results := make([]TelegramActionReceipt, 0, len(plan.Actions))
	completed := map[int]string{}
	for index, action := range plan.Actions {
		receipt := TelegramActionReceipt{ID: fmt.Sprintf("%s:%d:%d", userID, messageID, index), UserID: userID, TelegramMessageID: messageID, ActionIndex: index, ActionType: action.Type, Payload: action.Arguments}
		if blocked(action.DependsOn, completed) {
			receipt.Status, receipt.Result = "skipped", resultJSON("A required action did not apply.", nil)
		} else {
			receipt.Status, receipt.Result = agent.execute(ctx, userID, action, current)
			if receipt.Status == "applied" && action.Type != "show_budget" && action.Type != "clarify" && action.Type != "unsupported" {
				current, _ = agent.Store.GetTelegramBudget(ctx, userID)
			}
		}
		stored, err := agent.Store.SaveTelegramBudgetAction(ctx, receipt)
		if err != nil {
			return nil, fmt.Errorf("save action receipt: %w", err)
		}
		results = append(results, stored)
		completed[index] = stored.Status
	}
	return results, nil
}

func clarificationFor(message string) string {
	request := strings.ToLower(message)
	if strings.Contains(request, "split") && strings.Contains(request, "half") {
		return "Do you mean split your entire monthly budget, or split the current Other allocation? Please include the amount or category to split."
	}
	return "I couldn't safely interpret that request. Please break it into the category, amount, and destination you want to change."
}

// CurrentBudget returns the state to append after Telegram action receipts.
func (agent TelegramAgent) CurrentBudget(ctx context.Context, userID models.UserID) (*Budget, error) {
	return agent.Store.GetTelegramBudget(ctx, userID)
}

func (agent TelegramAgent) execute(ctx context.Context, userID models.UserID, action PlannedAction, current *Budget) (string, json.RawMessage) {
	switch action.Type {
	case "clarify", "unsupported":
		return action.Type, resultJSON(action.Explanation, nil)
	case "show_budget":
		if current == nil {
			return "failed", resultJSON("No budget exists yet. Send /budget followed by your monthly allocations to create one.", nil)
		}
		return "applied", resultJSON("Here is your current monthly budget.", current)
	case "create_budget", "replace_budget":
		var next Budget
		if err := json.Unmarshal(action.Arguments, &next); err != nil {
			return "failed", resultJSON("The budget action had invalid arguments.", nil)
		}
		if err := validate(ParseResult{Status: "valid", MonthlyTotalCents: next.MonthlyTotalCents, Categories: next.Categories}); err != nil {
			return "failed", resultJSON(err.Error(), nil)
		}
		if action.Type == "create_budget" && current != nil {
			return "failed", resultJSON("A budget already exists; request a change instead.", nil)
		}
		next.Version = 1
		if current != nil {
			next.Version = current.Version + 1
		}
		next.UpdatedAt = agent.now()
		if err := agent.Store.SaveTelegramBudget(ctx, userID, next); err != nil {
			return "failed", resultJSON("The budget could not be saved.", nil)
		}
		return "applied", resultJSON(fmt.Sprintf("Saved budget version %d.", next.Version), &next)
	case "move_allocation":
		if current == nil {
			return "failed", resultJSON("Create a budget before moving an allocation.", nil)
		}
		var move struct {
			From  string `json:"from"`
			To    string `json:"to"`
			Cents int64  `json:"cents"`
		}
		if err := json.Unmarshal(action.Arguments, &move); err != nil || move.Cents <= 0 {
			return "failed", resultJSON("The move action had invalid arguments.", nil)
		}
		next, err := moveAllocation(*current, move.From, move.To, move.Cents)
		if err != nil {
			return "failed", resultJSON(err.Error(), nil)
		}
		next.Version, next.UpdatedAt = current.Version+1, agent.now()
		if err := agent.Store.SaveTelegramBudget(ctx, userID, next); err != nil {
			return "failed", resultJSON("The budget change could not be saved.", nil)
		}
		return "applied", resultJSON(fmt.Sprintf("Moved $%.2f from %s to %s.", float64(move.Cents)/100, move.From, move.To), &next)
	default:
		return "unsupported", resultJSON("This action is not supported yet.", nil)
	}
}

func (agent TelegramAgent) now() time.Time {
	if agent.Now == nil {
		return time.Now().UTC()
	}
	return agent.Now().UTC()
}
func blocked(dependencies []int, completed map[int]string) bool {
	for _, dependency := range dependencies {
		if completed[dependency] != "applied" {
			return true
		}
	}
	return false
}
func resultJSON(summary string, budget *Budget) json.RawMessage {
	value, _ := json.Marshal(ActionResult{Summary: summary, Budget: budget})
	return value
}
func moveAllocation(current Budget, from, to string, cents int64) (Budget, error) {
	fromIndex := -1
	toIndex := -1
	for index := range current.Categories {
		if strings.EqualFold(current.Categories[index].Name, from) {
			fromIndex = index
		}
		if strings.EqualFold(current.Categories[index].Name, to) {
			toIndex = index
		}
	}
	if fromIndex < 0 {
		return Budget{}, fmt.Errorf("category %q does not exist", from)
	}
	if current.Categories[fromIndex].MonthlyLimitCents < cents {
		return Budget{}, fmt.Errorf("%s does not have $%.2f available", current.Categories[fromIndex].Name, float64(cents)/100)
	}
	next := current
	next.Categories = append([]Category(nil), current.Categories...)
	next.Categories[fromIndex].MonthlyLimitCents -= cents
	if toIndex >= 0 {
		next.Categories[toIndex].MonthlyLimitCents += cents
	} else {
		next.Categories = append(next.Categories, Category{Name: strings.TrimSpace(to), MonthlyLimitCents: cents})
	}
	if err := validate(ParseResult{Status: "valid", MonthlyTotalCents: next.MonthlyTotalCents, Categories: next.Categories}); err != nil {
		return Budget{}, err
	}
	return next, nil
}
