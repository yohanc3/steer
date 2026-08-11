package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	applicationbot "yohanc3/steer/bot"
	"yohanc3/steer/budget"
	"yohanc3/steer/config"
	"yohanc3/steer/models"
	"yohanc3/steer/storage"
	"yohanc3/steer/teller"

	telegrambot "github.com/go-telegram/bot"
	"github.com/joho/godotenv"
)

// newAPIServer registers HTTP routes for Telegram and Teller Connect.
// It requires the Telegram webhook handler and a fully configured Teller service.
func newAPIServer(telegramHandler http.Handler, tellerService teller.TellerService, budgetController budget.Controller) http.Handler {
	controller := tellerConnectController{tellerService: tellerService}
	mux := http.NewServeMux()

	// Route Telegram updates directly to the library-provided webhook handler.
	mux.Handle("POST /telegram/webhook", telegramHandler)

	// Route browser completion callbacks through the Teller controller.
	mux.HandleFunc("POST /api/teller/connect/complete", controller.complete)
	budgetController.Register(mux)

	// Expose a lightweight backend health endpoint for local orchestration.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}

// run constructs the application dependencies and serves HTTP until cancellation.
// It requires complete runtime configuration, SQLite access, Telegram, and Teller credentials.
func run(parent context.Context) error {
	// Load local development values when present; deployed environments provide them directly.
	_ = godotenv.Load()

	// Parse all required runtime configuration before starting any dependency.
	appConfig, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	// Open SQLite and apply pending migrations before serving any requests.
	database, err := storage.Open(parent, appConfig.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()
	// Reuse one repository implementation for the service's related stores.
	repository := storage.Repository{Database: database}
	budgetService := budget.Service{
		Store:        repository,
		AI:           budget.NewDeepSeekClient(appConfig.DeepSeekAPIKey, appConfig.DeepSeekModel),
		AIModel:      appConfig.DeepSeekModel,
		AIConfigured: appConfig.DeepSeekAPIKey != "",
		Now:          time.Now,
	}

	// Build the crypto and remote API dependencies needed for Teller operations.
	accessTokenCipher, err := teller.NewAESGCM([]byte(appConfig.TokenEncryptionKey))
	if err != nil {
		return err
	}
	tellerClient, err := teller.NewTellerHTTPClient(appConfig.TellerCertPEM, appConfig.TellerKeyPEM)
	if err != nil {
		return err
	}
	enrollmentVerifier, err := teller.NewEd25519EnrollmentVerifier(appConfig.TellerTokenSigningPublicKey)
	if err != nil {
		return fmt.Errorf("create Teller enrollment verifier: %w", err)
	}
	// Assemble the Teller workflow from its explicit persistence and security dependencies.
	tellerService := teller.TellerService{
		TellerClient:        tellerClient,
		UserStore:           repository,
		ConnectSessionStore: repository,
		TransactionStore:    repository,
		AccessTokenCipher:   accessTokenCipher,
		EnrollmentVerifier:  enrollmentVerifier,
		TellerEnvironment:   appConfig.TellerEnvironment,
	}

	// Configure Telegram commands before accepting webhook traffic.
	telegramBot, err := applicationbot.New(
		appConfig.TelegramBotToken,
		appConfig.TelegramWebhookSecret,
		applicationbot.ConnectController{
			UserStore:           repository,
			ConnectSessionStore: repository,
			PublicBaseURL:       appConfig.PublicBaseURL,
			Transactions:        applicationbot.TransactionsController{UserStore: repository, TransactionStore: repository, BalanceProvider: tellerService, Now: time.Now},
			BudgetAgent:         applicationbot.BudgetAgentController{UserStore: repository, Agent: budget.TelegramAgent{Store: repository, Planner: budget.NewDeepSeekClient(appConfig.DeepSeekAPIKey, appConfig.DeepSeekModel), Now: time.Now}},
		},
	)
	if err != nil {
		return err
	}
	if err := applicationbot.RegisterCommands(parent, telegramBot); err != nil {
		return err
	}
	// Cancel background work and begin HTTP shutdown when the process receives a signal.
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start independent webhook and polling loops before opening the HTTP listener.
	go telegramBot.StartWebhook(ctx)
	go poll(ctx, tellerService, repository, appConfig.TellerPollInterval)
	go backupDatabase(ctx, database, appConfig.DatabaseBackupDirectory, appConfig.DatabaseBackupInterval)

	// Serve public HTTP routes until the listener fails or shutdown is requested.
	server := &http.Server{Addr: ":8080", Handler: newAPIServer(telegramBot.WebhookHandler(), tellerService, budget.Controller{Service: budgetService}), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	errs := make(chan error, 1)
	go func() { errs <- server.ListenAndServe() }()
	go registerTelegramWebhook(ctx, telegramBot, appConfig.PublicBaseURL, appConfig.TelegramWebhookSecret)
	select {
	case serverErr := <-errs:
		if !errors.Is(serverErr, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", serverErr)
		}
	case <-ctx.Done():
	}
	// Bound graceful shutdown so process termination cannot wait indefinitely.
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

// registerTelegramWebhook retries until the public gateway is ready to receive Telegram updates.
func registerTelegramWebhook(ctx context.Context, telegramBot *telegrambot.Bot, publicBaseURL, secret string) {
	for {
		attempt, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := applicationbot.RegisterWebhook(attempt, telegramBot, publicBaseURL, secret)
		cancel()
		if err == nil {
			slog.Log(ctx, slog.LevelInfo, "registered Telegram webhook")
			return
		}
		slog.Log(ctx, slog.LevelWarn, "register Telegram webhook", "error", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// backupDatabase creates a daily snapshot while retaining the running application's database.
func backupDatabase(ctx context.Context, database *sql.DB, directory string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			path, err := storage.Backup(ctx, database, directory, now)
			if err != nil {
				slog.Log(ctx, slog.LevelError, "back up SQLite database", "error", err)
				continue
			}
			slog.Log(ctx, slog.LevelInfo, "backed up SQLite database", "path", path)
		}
	}

}

// poll periodically synchronizes each connected Teller account.
// It stops when ctx is cancelled and requires a configured TellerService and user store.
func poll(ctx context.Context, tellerService teller.TellerService, userStore models.UserStore, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// List active users before syncing them one at a time.
			connected, err := userStore.ListConnectedUsers(ctx)
			if err != nil {
				continue
			}

			// Isolate a failed account sync so other connected users still progress.
			for _, user := range connected {
				if err := tellerService.SyncUser(ctx, user.ID, false); err != nil {
					slog.Log(ctx, slog.LevelError, "poll teller account", "user_id", user.ID, "error", err)
				}
			}
		}
	}
}

// main starts the application using the process lifetime as its parent context.
// Startup failures are logged before the process exits unsuccessfully.
func main() {
	if err := run(context.Background()); err != nil {
		slog.Log(context.Background(), slog.LevelError, "application stopped", "error", err)
		os.Exit(1)
	}
}
