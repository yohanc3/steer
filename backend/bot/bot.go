// Package bot configures Telegram command handlers for the application.
package bot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
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

type balanceProvider interface {
	AvailableBalance(context.Context, models.UserID) (string, error)
}

var transactionPeriods = map[string]time.Duration{
	"transactions_24h": 24 * time.Hour,
	"transactions_3d":  3 * 24 * time.Hour,
	"transactions_7d":  7 * 24 * time.Hour,
	"transactions_30d": 30 * 24 * time.Hour,
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
	BalanceProvider  balanceProvider
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
	telegramBot.RegisterHandler(
		telegrambot.HandlerTypeMessageText,
		"connect",
		telegrambot.MatchTypeCommand,
		controller.connect,
	)
	for command, duration := range transactionPeriods {
		telegramBot.RegisterHandler(
			telegrambot.HandlerTypeMessageText,
			command,
			telegrambot.MatchTypeCommand,
			transactionsHandler(controller.Transactions, duration),
		)
	}
	return telegramBot, nil
}

// RegisterCommands publishes the bot's command menu to Telegram.
// It requires a live bot and a context that remains valid for the API request.
func RegisterCommands(ctx context.Context, telegramBot *telegrambot.Bot) error {
	// Publish the command so Telegram clients can surface it in the command menu.
	commands := []telegram.BotCommand{
		{Command: "connect", Description: "Connect a bank account"},
		{Command: "transactions_24h", Description: "Transactions from 24 hours"},
		{Command: "transactions_3d", Description: "Transactions from 3 days"},
		{Command: "transactions_7d", Description: "Transactions from 7 days"},
		{Command: "transactions_30d", Description: "Transactions from 30 days"},
	}
	_, err := telegramBot.SetMyCommands(ctx, &telegrambot.SetMyCommandsParams{Commands: commands})
	if err != nil {
		return fmt.Errorf("set telegram commands: %w", err)
	}
	return nil
}

func transactionsHandler(
	controller TransactionsController,
	duration time.Duration,
) telegrambot.HandlerFunc {
	return func(ctx context.Context, telegramBot *telegrambot.Bot, update *telegram.Update) {
		if update.Message == nil {
			return
		}
		user, err := controller.UserStore.GetOrCreateUser(ctx, update.Message.Chat.ID)
		if err != nil {
			slog.Log(ctx, slog.LevelError, "get transaction user", "error", err)
			return
		}
		transactions, err := controller.TransactionStore.ListTransactions(
			ctx,
			user.ID,
			controller.Now().Add(-duration),
		)
		if err != nil {
			slog.Log(ctx, slog.LevelError, "list transactions", "error", err)
			return
		}
		fields := strings.Fields(update.Message.Text)
		verbose := len(fields) > 1 && strings.EqualFold(fields[1], "verbose")
		var balance *string
		available, balanceErr := controller.BalanceProvider.AvailableBalance(ctx, user.ID)
		if balanceErr == nil {
			balance = &available
		} else {
			slog.Log(ctx, slog.LevelWarn, "get available balance", "error", balanceErr)
		}
		for _, text := range formatTransactionMessages(transactions, verbose, balance) {
			params := &telegrambot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: text}
			if !verbose {
				params.ParseMode = telegram.ParseModeHTML
			}
			if _, err := telegramBot.SendMessage(ctx, params); err != nil {
				slog.Log(ctx, slog.LevelError, "send transactions", "error", err)
				return
			}
		}
	}
}

func formatTransactionMessages(
	transactions []models.Transaction,
	verbose bool,
	availableBalance *string,
) []string {
	lines := make([]string, 0, len(transactions))
	if len(transactions) == 0 {
		lines = append(lines, "No transactions in this period.")
	}
	for _, transaction := range transactions {
		if verbose {
			lines = append(lines, strings.Join([]string{
				"ID: " + transaction.ID,
				"Account ID: " + transaction.AccountID,
				"Date: " + transaction.Date,
				"Amount: " + transaction.Amount,
				"Description: " + transaction.Description,
				"Status: " + transaction.Status,
				"Type: " + transaction.Type,
				"Running balance: " + optionalValue(transaction.RunningBalance),
				"Processing status: " + transaction.ProcessingStatus,
				"Category: " + optionalValue(transaction.Category),
				"Counterparty name: " + optionalValue(transaction.CounterpartyName),
				"Counterparty type: " + optionalValue(transaction.CounterpartyType),
				"Transaction link: " + transaction.SelfLink,
				"Account link: " + transaction.AccountLink,
			}, "\n"))
		} else {
			line := "<b>" + html.EscapeString(formatAmount(transaction.Amount)) + "</b>  "
			line += html.EscapeString(transaction.Description) + "\n<i>"
			line += html.EscapeString(formatDate(transaction.Date)) + " · "
			line += html.EscapeString(transaction.Status) + "</i>"
			lines = append(lines, line)
		}
	}
	if availableBalance != nil {
		if verbose {
			lines = append(lines, "Available balance: "+formatBalance(*availableBalance))
		} else {
			lines = append(
				lines,
				"<b>Available balance</b>\n"+html.EscapeString(formatBalance(*availableBalance)),
			)
		}
	}
	return splitTelegramMessages(lines)
}

func formatAmount(amount string) string {
	if strings.HasPrefix(amount, "-") {
		return "-$" + strings.TrimPrefix(amount, "-")
	}
	if strings.HasPrefix(amount, "+") {
		return "+$" + strings.TrimPrefix(amount, "+")
	}
	return "+$" + amount
}

func formatBalance(balance string) string { return "$" + balance }

func formatDate(value string) string {
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return value
	}
	return date.Format("Mon, Jan ") + ordinal(date.Day()) + date.Format(" 2006")
}

func ordinal(day int) string {
	suffix := "th"
	if day%100 < 11 || day%100 > 13 {
		switch day % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", day, suffix)
}

func optionalValue(value *string) string {
	if value == nil {
		return "not available"
	}
	return *value
}

func splitTelegramMessages(lines []string) []string {
	const maximumLength = 4096
	var messages []string
	var current strings.Builder
	for _, line := range lines {
		if current.Len() > 0 && current.Len()+1+len(line) > maximumLength {
			messages = append(messages, current.String())
			current.Reset()
		}
		for len(line) > maximumLength {
			if current.Len() > 0 {
				messages = append(messages, current.String())
				current.Reset()
			}
			messages = append(messages, line[:maximumLength])
			line = line[maximumLength:]
		}
		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)
	}
	if current.Len() > 0 {
		messages = append(messages, current.String())
	}
	return messages
}

// RegisterWebhook tells Telegram to deliver updates through the public Nginx gateway.
func RegisterWebhook(
	ctx context.Context,
	telegramBot webhookRegistrar,
	publicBaseURL, secret string,
) error {
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
func (controller ConnectController) connect(
	ctx context.Context,
	telegramBot *telegrambot.Bot,
	update *telegram.Update,
) {
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
		err = controller.ConnectSessionStore.CreateConnectSession(ctx, models.ConnectSession{
			TokenHash: storage.HashToken(token),
			UserID:    user.ID,
			Nonce:     nonce,
			ExpiresAt: time.Now().Add(15 * time.Minute),
		})
	}

	if err != nil {
		slog.Log(
			ctx,
			slog.LevelError,
			"create teller session",
			slog.String("user_id", string(user.ID)),
			"error",
			err,
		)
		return
	}

	// Send both token and nonce so Teller Connect can return the signed enrollment.
	slog.Log(ctx, slog.LevelInfo, "created teller connect session", "user_id", user.ID)
	if _, err := telegramBot.SendMessage(ctx, &telegrambot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   controller.PublicBaseURL + "/connect?session=" + token + "&nonce=" + nonce,
	}); err != nil {
		slog.Log(
			ctx,
			slog.LevelError,
			"error when sending teller connect link",
			"chat_id",
			update.Message.Chat.ID,
			"error",
			err,
		)
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
