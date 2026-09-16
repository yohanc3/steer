package main

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	applog "yohanc3/steer/logger"
	"yohanc3/steer/telegrambot"
)

type fakeTelegramConnectionService struct {
	code   *telegrambot.Code
	err    error
	userID string
}

func (service *fakeTelegramConnectionService) GetConnectionCode(_ context.Context, userID string) (*telegrambot.Code, error) {
	service.userID = userID
	return service.code, service.err
}

func testLogger() *applog.Logger {
	return &applog.Logger{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func testDatabase(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return db
}

func TestNewServerInjectsProtectedAndPublicDependencies(t *testing.T) {
	telegramService := &fakeTelegramConnectionService{
		code: &telegrambot.Code{Code: "test-code", ExpiresAt: 12345},
	}

	server, err := NewServer(ServerDependencies{
		Logger:          testLogger(),
		DB:              testDatabase(t),
		TelegramService: telegramService,
		AuthMiddleware: func(next http.Handler) (http.Handler, error) {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Test-Auth", "applied")
				next.ServeHTTP(w, r)
			}), nil
		},
		RegisterPublicRoutes: func(mux *http.ServeMux) error {
			mux.HandleFunc("POST /bot", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			return nil
		},
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	apiRequest := httptest.NewRequest(http.MethodGet, "/api/user/user-1/telegram/connection-code", nil)
	apiResponse := httptest.NewRecorder()
	server.ServeHTTP(apiResponse, apiRequest)

	if apiResponse.Code != http.StatusOK {
		t.Fatalf("expected API status 200, got %d", apiResponse.Code)
	}
	if apiResponse.Header().Get("X-Test-Auth") != "applied" {
		t.Fatal("expected injected auth middleware to wrap API routes")
	}
	if telegramService.userID != "user-1" {
		t.Fatalf("expected injected Telegram service to receive user-1, got %q", telegramService.userID)
	}

	botRequest := httptest.NewRequest(http.MethodPost, "/bot", nil)
	botResponse := httptest.NewRecorder()
	server.ServeHTTP(botResponse, botRequest)

	if botResponse.Code != http.StatusNoContent {
		t.Fatalf("expected public route status 204, got %d", botResponse.Code)
	}
	if botResponse.Header().Get("X-Test-Auth") != "" {
		t.Fatal("expected public routes to remain outside auth middleware")
	}
}

func TestNewServerReturnsInjectedDependencyErrors(t *testing.T) {
	wantErr := errors.New("auth setup failed")

	_, err := NewServer(ServerDependencies{
		Logger:          testLogger(),
		DB:              testDatabase(t),
		TelegramService: &fakeTelegramConnectionService{},
		AuthMiddleware: func(http.Handler) (http.Handler, error) {
			return nil, wantErr
		},
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected injected error %v, got %v", wantErr, err)
	}
}
