package budget

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const deepSeekEndpoint = "https://api.deepseek.com/chat/completions"

// DeepSeekClient calls DeepSeek's OpenAI-compatible JSON output endpoint.
type DeepSeekClient struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// NewDeepSeekClient constructs a client that uses the low-latency model unless configured otherwise.
func NewDeepSeekClient(apiKey, model string) *DeepSeekClient {
	if model == "" {
		model = "deepseek-v4-flash"
	}
	return &DeepSeekClient{APIKey: apiKey, Model: model, HTTPClient: &http.Client{Timeout: 20 * time.Second}}
}

// ParseBudget asks the model to either produce a complete budget JSON object or explain its gap.
func (client *DeepSeekClient) ParseBudget(ctx context.Context, instruction string) (ParseResult, error) {
	var result ParseResult
	err := client.completeJSON(ctx, []chatMessage{
		{Role: "system", Content: `You parse a user's monthly budget instruction. Return JSON only, exactly one of:
{"status":"valid","monthly_total_cents":170000,"categories":[{"name":"Rent","monthly_limit_cents":80000}]}
or {"status":"invalid","reason":"brief explanation"}.
The input may include a current saved budget JSON followed by a requested change. In that case, return the complete revised budget: retain unchanged categories and apply only the explicit change. Never infer missing amounts, categories, or a monthly total. Convert dollars to integer cents. Categories must add exactly to the total. "Unallocated" is a valid explicit category. Do not include markdown or fields beyond this JSON contract.`},
		{Role: "user", Content: instruction},
	}, &result)
	if err != nil {
		return ParseResult{}, err
	}
	if result.Status != "valid" && result.Status != "invalid" {
		return ParseResult{}, fmt.Errorf("%w: model returned an unsupported budget status", ErrAIUnavailable)
	}
	return result, nil
}

// ClassifyTransaction chooses an existing budget category or explicitly returns unmatched.
func (client *DeepSeekClient) ClassifyTransaction(ctx context.Context, transaction Transaction, categories []Category) (string, error) {
	categoryJSON, err := json.Marshal(categories)
	if err != nil {
		return "", fmt.Errorf("encode budget categories: %w", err)
	}
	var result struct {
		Category string `json:"category"`
	}
	err = client.completeJSON(ctx, []chatMessage{
		{Role: "system", Content: `You map a transaction to a user's existing budget category. Return JSON only in this exact shape: {"category":"one exact category name or unmatched"}. Choose an exact provided category name only when the match is clear. If no clear fit exists, return "unmatched". Never create or infer a category.`},
		{Role: "user", Content: fmt.Sprintf("Transaction merchant: %q\nProvider category: %q\nAmount cents: %d\nAllowed categories JSON: %s", transaction.Merchant, transaction.ProviderCategory, transaction.AmountCents, categoryJSON)},
	}, &result)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Category), nil
}

// PlanTelegramBudgetActions turns one Telegram request into only supported, typed actions.
func (client *DeepSeekClient) PlanTelegramBudgetActions(ctx context.Context, message string, current *Budget) (ActionPlan, error) {
	currentJSON := "null"
	if current != nil {
		encoded, err := json.Marshal(current)
		if err != nil {
			return ActionPlan{}, fmt.Errorf("encode current budget: %w", err)
		}
		currentJSON = string(encoded)
	}
	var plan ActionPlan
	err := client.completeJSON(ctx, []chatMessage{
		{Role: "system", Content: `You are a budget action planner. Return JSON only: {"actions":[...]}. Each action has type, depends_on_indexes, arguments, and explanation. Actions are zero-indexed by their position in the actions array; use depends_on_indexes only when an action requires an earlier action to apply. Supported types: create_budget and replace_budget with arguments {"monthly_total_cents":integer,"categories":[{"name":string,"monthly_limit_cents":integer}]}; move_allocation with {"from":string,"to":string,"cents":integer}; show_budget with {}; clarify with {} and a concise explanation; unsupported with {} and a concise explanation. Use cents. For one message with multiple requests, output separate independent actions. Do not invent missing amounts. A move can create its destination category. Never output database operations, markdown, IDs, or unsupported action types.`},
		{Role: "user", Content: "Current budget JSON:\n" + currentJSON + "\n\nTelegram request:\n" + message},
	}, &plan)
	if err != nil {
		return ActionPlan{}, err
	}
	if len(plan.Actions) == 0 {
		return ActionPlan{}, fmt.Errorf("%w: model returned no actions", ErrAIUnavailable)
	}
	return plan, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (client *DeepSeekClient) completeJSON(ctx context.Context, messages []chatMessage, target any) error {
	if strings.TrimSpace(client.APIKey) == "" {
		return fmt.Errorf("%w: set DEEPSEEK_API_KEY before using the prototype", ErrAIUnavailable)
	}
	body, err := json.Marshal(struct {
		Model          string        `json:"model"`
		Messages       []chatMessage `json:"messages"`
		ResponseFormat struct {
			Type string `json:"type"`
		} `json:"response_format"`
		MaxTokens int `json:"max_tokens"`
	}{
		Model:     client.Model,
		Messages:  messages,
		MaxTokens: 700,
		ResponseFormat: struct {
			Type string `json:"type"`
		}{Type: "json_object"},
	})
	if err != nil {
		return fmt.Errorf("encode DeepSeek request: %w", err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, deepSeekEndpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create DeepSeek request: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+client.APIKey)
		request.Header.Set("Content-Type", "application/json")
		response, err := client.HTTPClient.Do(request)
		if err != nil {
			return fmt.Errorf("%w: call DeepSeek: %v", ErrAIUnavailable, err)
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read DeepSeek response: %w", readErr)
		}
		if response.StatusCode >= http.StatusInternalServerError && attempt < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
				continue
			}
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return fmt.Errorf("%w: DeepSeek returned %s", ErrAIUnavailable, response.Status)
		}
		var completion struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(responseBody, &completion); err != nil {
			return fmt.Errorf("decode DeepSeek response: %w", err)
		}
		if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
			return fmt.Errorf("%w: DeepSeek returned no JSON content", ErrAIUnavailable)
		}
		if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), target); err != nil {
			return fmt.Errorf("%w: decode model JSON: %v", ErrAIUnavailable, err)
		}
		return nil
	}
	return fmt.Errorf("%w: DeepSeek retry budget exhausted", ErrAIUnavailable)
}
