// Package bot configures Telegram command handlers for the application.
package bot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	telegrambot "github.com/go-telegram/bot"
	telegram "github.com/go-telegram/bot/models"

	"yohanc3/steer/models"
	"yohanc3/steer/storage"
)

// ConnectController creates one-time Teller Connect links for Telegram users.
// Users and sessions must share the same persistent storage implementation.
type ConnectController struct {
	UserStore           models.UserStore
	ConnectSessionStore models.ConnectSessionStore
	PublicBaseURL       string
}

// New creates a Telegram bot and registers the supported application commands.
// The token and webhook secret must come from Telegram's BotFather configuration.
func New(botToken, webhookSecret string, controller ConnectController) (*telegrambot.Bot, error) {
	telegramBot, err := telegrambot.New(botToken, telegrambot.WithWebhookSecretToken(webhookSecret))
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}
	telegramBot.RegisterHandler(telegrambot.HandlerTypeMessageText, "connect", telegrambot.MatchTypeCommand, controller.connect)
	return telegramBot, nil
}

// RegisterCommands publishes the bot's command menu to Telegram.
// It requires a live bot and a context that remains valid for the API request.
func RegisterCommands(ctx context.Context, telegramBot *telegrambot.Bot) error {
	_, err := telegramBot.SetMyCommands(ctx, &telegrambot.SetMyCommandsParams{Commands: []telegram.BotCommand{{Command: "connect", Description: "Connect a bank account"}}})
	if err != nil {
		return fmt.Errorf("set telegram commands: %w", err)
	}
	return nil
}

// connect creates an expiring browser link for a Telegram /connect command.
// The update must contain a chat message and configured persistent stores.
func (controller ConnectController) connect(ctx context.Context, telegramBot *telegrambot.Bot, update *telegram.Update) {
	if update.Message == nil {
		return
	}

	slog.Log(ctx, slog.LevelInfo, "handle connect command", "chat_id", update.Message.Chat.ID)
	user, err := controller.UserStore.GetOrCreateUser(ctx, update.Message.Chat.ID)
	if err != nil {
		slog.Log(ctx, slog.LevelError, "error when creating telegram user", "error", err)
		return
	}

	token, nonce, err := newConnectSession()
	if err == nil {
		err = controller.ConnectSessionStore.CreateConnectSession(ctx, models.ConnectSession{TokenHash: storage.HashToken(token), UserID: user.ID, Nonce: nonce, ExpiresAt: time.Now().Add(15 * time.Minute)})
	}

	if err != nil {
		slog.Log(ctx, slog.LevelError, "create teller session", slog.String("user_id", string(user.ID)), "error", err)
		return
	}

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
