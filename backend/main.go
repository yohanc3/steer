package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	applicationbot "yohanc3/steer/bot"
	"yohanc3/steer/config"
	"yohanc3/steer/models"
	"yohanc3/steer/storage"
	"yohanc3/steer/teller"

	"github.com/joho/godotenv"
)

// newAPIServer registers HTTP routes for Telegram and Teller Connect.
// It requires the Telegram webhook handler and a fully configured Teller service.
func newAPIServer(telegramHandler http.Handler, service teller.TellerService) http.Handler {
	controller := tellerConnectController{service: service}
	mux := http.NewServeMux()
	mux.Handle("POST /telegram/webhook", telegramHandler)
	mux.HandleFunc("POST /api/teller/connect/complete", controller.complete)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}

// run constructs the application dependencies and serves HTTP until cancellation.
// It requires complete runtime configuration, SQLite access, Telegram, and Teller credentials.
func run(parent context.Context) error {
	_ = godotenv.Load()
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	db, err := storage.Open(parent, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	repository := storage.Repository{DB: db}
	cipher, err := teller.NewAESGCM([]byte(cfg.TokenEncryptionKey))
	if err != nil {
		return err
	}
	client, err := teller.NewHTTPClient(cfg.TellerCertPEM, cfg.TellerKeyPEM)
	if err != nil {
		return err
	}
	verifier, err := teller.NewEd25519EnrollmentVerifier(cfg.TellerTokenSigningPublicKey)
	if err != nil {
		return fmt.Errorf("create Teller enrollment verifier: %w", err)
	}
	service := teller.TellerService{Client: client, Users: repository, Sessions: repository, Transactions: repository, Cipher: cipher, Verifier: verifier, Environment: cfg.TellerEnvironment}
	b, err := applicationbot.New(cfg.TelegramBotToken, cfg.TelegramWebhookSecret, applicationbot.ConnectController{Users: repository, Sessions: repository, PublicBaseURL: cfg.PublicBaseURL})
	if err != nil {
		return err
	}
	if err := applicationbot.RegisterCommands(parent, b); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	go b.StartWebhook(ctx)
	go poll(ctx, service, repository, cfg.TellerPollInterval)
	server := &http.Server{Addr: ":8080", Handler: newAPIServer(b.WebhookHandler(), service), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	errs := make(chan error, 1)
	go func() { errs <- server.ListenAndServe() }()
	select {
	case serverErr := <-errs:
		if !errors.Is(serverErr, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", serverErr)
		}
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

// poll periodically synchronizes each connected Teller account.
// It stops when ctx is cancelled and requires a configured TellerService and user store.
func poll(ctx context.Context, service teller.TellerService, users models.UserStore, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			connected, err := users.ListConnectedUsers(ctx)
			if err != nil {
				continue
			}
			for _, user := range connected {
				if err := service.SyncUser(ctx, user.ID, false); err != nil {
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
