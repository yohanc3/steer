// Package jev integrates TypeSafe's System One Choice API for transaction categorization.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"yohanc3/steer/models"
)

const (
	defaultBaseURL       = "https://api.typesafe.ai"
	systemOneModel       = "jev-latest"
	uncategorizedOption  = "uncategorized"
	categoryQuestionID   = "transaction_category"
	maximumChoiceOptions = 255
)

// Client calls TypeSafe without automatic retries; the scheduled worker owns reprocessing policy.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient builds a server-side TypeSafe client using the required bearer credential.
func NewClient(apiKey string) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("TypeSafe API key is required")
	}
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
	}, nil
}

// Categorize asks one Choice question over the complete user-owned category set.
func (client *Client) Categorize(ctx context.Context, transaction models.Transaction, categories []models.TransactionCategory) (models.CategoryDecision, error) {
	if len(categories) == 0 {
		return models.CategoryDecision{}, fmt.Errorf("at least one transaction category is required")
	}
	if len(categories)+1 > maximumChoiceOptions {
		return models.CategoryDecision{}, fmt.Errorf("too many transaction categories: %d", len(categories))
	}

	criteria := make(map[string]any, len(categories)+1)
	optionCategoryIDs := make(map[string]string, len(categories))
	for index, category := range categories {
		option := fmt.Sprintf("category_%d", index)
		criteria[option] = map[string]string{
			"name":        category.Name,
			"description": category.Description,
		}
		optionCategoryIDs[option] = category.ID
	}
	criteria[uncategorizedOption] = map[string]string{
		"name":        "Uncategorized",
		"description": "Choose this when none of the user's categories clearly fits the transaction.",
	}

	payload := map[string]any{
		"model": systemOneModel,
		"state": map[string]any{
			"description":       transaction.Description,
			"counterparty_name": transaction.CounterpartyName,
			"provider_category": transaction.Category,
			"transaction_type":  transaction.Type,
			"amount":            transaction.Amount,
		},
		"questions": map[string]any{
			categoryQuestionID: map[string]any{
				"type":         "choice",
				"instructions": "Which one of this user's categories best describes the transaction? Choose Uncategorized unless a category clearly fits.",
				"criteria":     criteria,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return models.CategoryDecision{}, fmt.Errorf("encode TypeSafe request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(client.baseURL, "/")+"/v1/systemone", bytes.NewReader(body))
	if err != nil {
		return models.CategoryDecision{}, fmt.Errorf("build TypeSafe request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return models.CategoryDecision{}, fmt.Errorf("perform TypeSafe request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return models.CategoryDecision{}, fmt.Errorf("TypeSafe response: %s", response.Status)
	}

	var result struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Type          string             `json:"type"`
			Choice        string             `json:"choice"`
			Confidence    float64            `json:"confidence"`
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return models.CategoryDecision{}, fmt.Errorf("decode TypeSafe response: %w", err)
	}
	answer, exists := result.Answers[categoryQuestionID]
	if !exists || result.Model == "" || answer.Type != "choice" || answer.Confidence < 0 || answer.Confidence > 1 {
		return models.CategoryDecision{}, fmt.Errorf("invalid TypeSafe category answer")
	}

	decision := models.CategoryDecision{
		Confidence:    answer.Confidence,
		Model:         result.Model,
		Probabilities: make(map[string]float64, len(answer.Probabilities)),
	}
	for option, probability := range answer.Probabilities {
		if option == uncategorizedOption {
			decision.Probabilities[uncategorizedOption] = probability
			continue
		}
		categoryID, valid := optionCategoryIDs[option]
		if !valid {
			return models.CategoryDecision{}, fmt.Errorf("TypeSafe returned an unknown category option %q", option)
		}
		decision.Probabilities[categoryID] = probability
	}
	if answer.Choice == uncategorizedOption {
		return decision, nil
	}
	categoryID, valid := optionCategoryIDs[answer.Choice]
	if !valid {
		return models.CategoryDecision{}, fmt.Errorf("TypeSafe chose an unknown category option %q", answer.Choice)
	}
	decision.CategoryID = &categoryID
	return decision, nil
}
