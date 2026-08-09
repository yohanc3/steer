// Package bot configures Telegram command handlers for the application.
package bot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	telegrambot "github.com/go-telegram/bot"
	telegram "github.com/go-telegram/bot/models"

	"yohanc3/steer/models"
	"yohanc3/steer/storage"
)

type webhookRegistrar interface {
	SetWebhook(context.Context, *telegrambot.SetWebhookParams) (bool, error)
}

// ConnectController creates one-time Teller Connect links for Telegram users.
// Users and sessions must share the same persistent storage implementation.
type ConnectController struct {
	UserStore           models.UserStore
	ConnectSessionStore models.ConnectSessionStore
	PublicBaseURL       string
	Transactions        TransactionsController
}

// TransactionsController renders recent locally synchronized transactions for a Telegram user.
type TransactionsController struct {
	UserStore        models.UserStore
	TransactionStore models.TransactionStore
	Now              func() time.Time
}

// New creates a Telegram bot and registers the supported application commands.
// The token and webhook secret must come from Telegram's BotFather configuration.
func New(botToken, webhookSecret string, controller ConnectController) (*telegrambot.Bot, error) {
	// Attach Telegram's shared-secret validation before registering commands.
	telegramBot, err := telegrambot.New(botToken, telegrambot.WithWebhookSecretToken(webhookSecret))
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	// Match the standard /connect command and delegate its workflow to the controller.
	telegramBot.RegisterHandler(telegrambot.HandlerTypeMessageText, "connect", telegrambot.MatchTypeCommand, controller.connect)
	for command, duration := range map[string]time.Duration{"transactions_24h": 24 * time.Hour, "transactions_3d": 72 * time.Hour, "transactions_7d": 7 * 24 * time.Hour, "transactions_30d": 30 * 24 * time.Hour} {
		telegramBot.RegisterHandler(telegrambot.HandlerTypeMessageText, command, telegrambot.MatchTypeCommand, transactionsHandler(controller.Transactions, duration))
	}
	return telegramBot, nil
}

// RegisterCommands publishes the bot's command menu to Telegram.
// It requires a live bot and a context that remains valid for the API request.
func RegisterCommands(ctx context.Context, telegramBot *telegrambot.Bot) error {
	// Publish the command so Telegram clients can surface it in the command menu.
	_, err := telegramBot.SetMyCommands(ctx, &telegrambot.SetMyCommandsParams{Commands: []telegram.BotCommand{{Command: "connect", Description: "Connect a bank account"}, {Command: "transactions_24h", Description: "Transactions from 24 hours"}, {Command: "transactions_3d", Description: "Transactions from 3 days"}, {Command: "transactions_7d", Description: "Transactions from 7 days"}, {Command: "transactions_30d", Description: "Transactions from 30 days"}}})
	if err != nil {
		return fmt.Errorf("set telegram commands: %w", err)
	}
	return nil
}

func transactionsHandler(controller TransactionsController, duration time.Duration) telegrambot.HandlerFunc {
	return func(ctx context.Context, telegramBot *telegrambot.Bot, update *telegram.Update) {
		if update.Message == nil {
			return
		}
		user, err := controller.UserStore.GetOrCreateUser(ctx, update.Message.Chat.ID)
		if err != nil {
			slog.Log(ctx, slog.LevelError, "get transaction user", "error", err)
			return
		}
		transactions, err := controller.TransactionStore.ListTransactions(ctx, user.ID, controller.Now().Add(-duration))
		if err != nil {
			slog.Log(ctx, slog.LevelError, "list transactions", "error", err)
			return
		}
		text := "No transactions in this period."
		if len(transactions) > 0 {
			var lines []string
			for _, transaction := range transactions {
				lines = append(lines, transaction.Date+" · "+transaction.Amount+" · "+transaction.Description)
			}
			text = strings.Join(lines, "\n")
		}
		if _, err := telegramBot.SendMessage(ctx, &telegrambot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: text}); err != nil {
			slog.Log(ctx, slog.LevelError, "send transactions", "error", err)
		}
	}
}

// RegisterWebhook tells Telegram to deliver updates through the public Nginx gateway.
func RegisterWebhook(ctx context.Context, telegramBot webhookRegistrar, publicBaseURL, secret string) error {
	registered, err := telegramBot.SetWebhook(ctx, &telegrambot.SetWebhookParams{
		URL:            publicBaseURL + "/telegram/webhook",
		SecretToken:    secret,
		AllowedUpdates: []string{"message"},
	})
	if err != nil {
		return fmt.Errorf("set Telegram webhook: %w", err)
	}
	if !registered {
		return fmt.Errorf("set Telegram webhook: Telegram rejected the request")
	}
	return nil
}

// connect creates an expiring browser link for a Telegram /connect command.
// The update must contain a chat message and configured persistent stores.
func (controller ConnectController) connect(ctx context.Context, telegramBot *telegrambot.Bot, update *telegram.Update) {
	// Ignore non-message updates because they have no conversation to link.
	if update.Message == nil {
		return
	}

	// Resolve the local user before issuing a one-time browser session.
	slog.Log(ctx, slog.LevelInfo, "handle connect command", "chat_id", update.Message.Chat.ID)
	user, err := controller.UserStore.GetOrCreateUser(ctx, update.Message.Chat.ID)
	if err != nil {
		slog.Log(ctx, slog.LevelError, "error when creating telegram user", "error", err)
		return
	}

	// Generate and store a short-lived token that binds the browser to this user.
	token, nonce, err := newConnectSession()
	if err == nil {
		err = controller.ConnectSessionStore.CreateConnectSession(ctx, models.ConnectSession{TokenHash: storage.HashToken(token), UserID: user.ID, Nonce: nonce, ExpiresAt: time.Now().Add(15 * time.Minute)})
	}

	if err != nil {
		slog.Log(ctx, slog.LevelError, "create teller session", slog.String("user_id", string(user.ID)), "error", err)
		return
	}

	// Send both token and nonce so Teller Connect can return the signed enrollment.
	slog.Log(ctx, slog.LevelInfo, "created teller connect session", "user_id", user.ID)
	if _, err := telegramBot.SendMessage(ctx, &telegrambot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   controller.PublicBaseURL + "/connect?session=" + token + "&nonce=" + nonce,
	}); err != nil {
		slog.Log(ctx, slog.LevelError, "error when sending teller connect link", "chat_id", update.Message.Chat.ID, "error", err)
		return
	}

	slog.Log(ctx, slog.LevelInfo, "sent teller connect link", "chat_id", update.Message.Chat.ID)
}

// newConnectSession creates independent random values for the URL token and nonce.
// It requires cryptographic randomness from the operating system.
func newConnectSession() (string, string, error) {
	// Generate independent high-entropy values for bearer-token and signature uses.
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
