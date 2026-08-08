package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"yohanc3/steer/config"
	"yohanc3/steer/models"
	"yohanc3/steer/storage"
	"yohanc3/steer/teller"

	"github.com/go-telegram/bot"
	telegram "github.com/go-telegram/bot/models"
	"github.com/joho/godotenv"
)

func newHandler(telegramHandler http.Handler, complete func(http.ResponseWriter, *http.Request)) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /telegram/webhook", telegramHandler)
	mux.HandleFunc("POST /api/teller/connect/complete", complete)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}

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
	service := teller.Service{Client: client, Users: repository, Sessions: repository, Transactions: repository, Cipher: cipher, Verifier: verifier, Environment: cfg.TellerEnvironment}
	b, err := bot.New(cfg.TelegramBotToken, bot.WithWebhookSecretToken(cfg.TelegramWebhookSecret))
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}
	b.RegisterHandler(bot.HandlerTypeMessageText, "/connect", bot.MatchTypeCommand, func(ctx context.Context, b *bot.Bot, update *telegram.Update) {
		if update.Message == nil {
			return
		}
		user, err := repository.GetOrCreateUser(ctx, update.Message.Chat.ID)
		if err != nil {
			slog.Log(ctx, slog.LevelError, "create telegram user", "error", err)
			return
		}
		token, nonce, err := newConnectSession()
		if err == nil {
			err = repository.CreateConnectSession(ctx, models.ConnectSession{TokenHash: storage.HashToken(token), UserID: user.ID, Nonce: nonce, ExpiresAt: time.Now().Add(15 * time.Minute)})
		}
		if err != nil {
			slog.Log(ctx, slog.LevelError, "create teller session", "error", err)
			return
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: cfg.PublicBaseURL + "/connect?session=" + token + "&nonce=" + nonce})
	})
	if _, err := b.SetMyCommands(parent, &bot.SetMyCommandsParams{Commands: []telegram.BotCommand{{Command: "connect", Description: "Connect a bank account"}}}); err != nil {
		return fmt.Errorf("set telegram commands: %w", err)
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	go b.StartWebhook(ctx)
	go poll(ctx, service, repository, cfg.TellerPollInterval)
	complete := func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			SessionToken string `json:"session_token"`
			AccessToken  string `json:"access_token"`
			Enrollment   struct {
				ID string `json:"id"`
			} `json:"enrollment"`
			User struct {
				ID string `json:"id"`
			} `json:"user"`
			Signatures []string `json:"signatures"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if err := service.Complete(r.Context(), request.SessionToken, request.AccessToken, request.Enrollment.ID, request.User.ID, request.Signatures); err != nil {
			slog.Log(r.Context(), slog.LevelError, "complete teller connection", "error", err)
			http.Error(w, "unable to connect account", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
	server := &http.Server{Addr: ":8080", Handler: newHandler(b.WebhookHandler(), complete), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
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

func poll(ctx context.Context, service teller.Service, users models.UserStore, interval time.Duration) {
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
func newConnectSession() (string, string, error) {
	raw := make([]byte, 32)
	nonce := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}
	return hex.EncodeToString(raw), hex.EncodeToString(nonce), nil
}
func main() {
	if err := run(context.Background()); err != nil {
		slog.Log(context.Background(), slog.LevelError, "application stopped", "error", err)
		os.Exit(1)
	}
}
