package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorsAllowsConfiguredOrigin(t *testing.T) {
	handler := AddCorsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), []string{"https://steer.example.com"})

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "https://steer.example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://steer.example.com" {
		t.Fatalf("expected configured origin, got %q", got)
	}
}

func TestCorsRejectsUnknownOrigin(t *testing.T) {
	handler := AddCorsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), []string{"https://steer.example.com"})

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected unknown origin to be rejected, got %q", got)
	}
}
