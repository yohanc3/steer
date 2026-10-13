package jev

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yohanc3/steer/models"
)

func TestCategorizeUsesTypeSafeChoiceAndTranslatesCategoryIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/systemone" {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		var payload struct {
			Model     string         `json:"model"`
			State     map[string]any `json:"state"`
			Questions map[string]struct {
				Type     string         `json:"type"`
				Criteria map[string]any `json:"criteria"`
			} `json:"questions"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		question := payload.Questions[categoryQuestionID]
		if payload.Model != systemOneModel || question.Type != "choice" {
			t.Errorf("payload = %#v", payload)
		}
		if payload.State["description"] != "Corner Store" || question.Criteria[uncategorizedOption] == nil || question.Criteria["category_1"] == nil {
			t.Errorf("payload = %#v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"model":"jev-current-build","answers":{"transaction_category":{"type":"choice","choice":"category_1","confidence":0.91,"probabilities":{"category_0":0.04,"category_1":0.91,"uncategorized":0.05}}}}`))
	}))
	defer server.Close()

	client, err := NewClient("secret")
	if err != nil {
		t.Fatal(err)
	}
	client.baseURL = server.URL
	client.httpClient = server.Client()
	categories := []models.TransactionCategory{{ID: "fuel-id", Name: "Fuel"}, {ID: "groceries-id", Name: "Groceries", Description: "Food bought for home"}}
	decision, err := client.Categorize(context.Background(), models.Transaction{Description: "Corner Store", Amount: "-12.00"}, categories)
	if err != nil {
		t.Fatal(err)
	}
	if decision.CategoryID == nil || *decision.CategoryID != "groceries-id" || decision.Confidence != 0.91 || decision.Model != "jev-current-build" {
		t.Fatalf("decision = %#v", decision)
	}
	if decision.Probabilities["groceries-id"] != 0.91 || decision.Probabilities[uncategorizedOption] != 0.05 {
		t.Fatalf("probabilities = %#v", decision.Probabilities)
	}
}

func TestCategorizeAllowsUncategorizedAndRejectsBadResponses(t *testing.T) {
	responses := []struct {
		name      string
		status    int
		body      string
		wantError bool
	}{
		{name: "uncategorized", status: http.StatusOK, body: `{"model":"jev","answers":{"transaction_category":{"type":"choice","choice":"uncategorized","confidence":0.8,"probabilities":{"category_0":0.2,"uncategorized":0.8}}}}`},
		{name: "unknown choice", status: http.StatusOK, body: `{"model":"jev","answers":{"transaction_category":{"type":"choice","choice":"other","confidence":0.8,"probabilities":{"category_0":0.2,"other":0.8}}}}`, wantError: true},
		{name: "service failure", status: http.StatusTooManyRequests, body: `{}`, wantError: true},
	}
	for _, test := range responses {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := NewClient("secret")
			if err != nil {
				t.Fatal(err)
			}
			client.baseURL = server.URL
			client.httpClient = server.Client()
			decision, err := client.Categorize(context.Background(), models.Transaction{}, []models.TransactionCategory{{ID: "category-id", Name: "Category"}})
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v", err)
			}
			if !test.wantError && decision.CategoryID != nil {
				t.Fatalf("decision = %#v", decision)
			}
		})
	}
}
