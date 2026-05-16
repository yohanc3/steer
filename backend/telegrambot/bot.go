package telegrambot

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"yohanc3/steer/config"
	applog "yohanc3/steer/logger"

	"github.com/mymmrac/telego"
)

// Handles updates, which are usually messages (hence the function name)
func handleUserMessage(ctx context.Context, update *telego.Update, bot *telego.Bot, telegramService *TelegramService, logger *applog.Logger) {

	userText := update.Message.Text
	responseText := ""

	var err error

	switch {
		case strings.HasPrefix(userText, "/start"):
			
			code := strings.Replace(userText, "/start ", "", 1)
			err = telegramService.DeepLinkAccount(ctx, code, update.Message.Chat.ID)
			responseText = "Account succesfully linked!"

		default:
			responseText = userText
	}

	if err != nil {
		err = fmt.Errorf("error when handling user message: %w", err)
		responseText = "Something went wrong. Try again later." 
		logger.Error(err.Error())
	}

	bot.SendMessage(ctx, &telego.SendMessageParams{ChatID: update.Message.Chat.ChatID(), Text: responseText})
	
	return
}

// Initializes a worker which will handle updates as they come. See handleUserMessage()
func Worker(ctx context.Context, id uint16, updates <-chan telego.Update, bot *telego.Bot, logger *applog.Logger, telegramService *TelegramService) {
	for update := range updates {
		logger.Debug(fmt.Sprintf("Worker %d processing update with id %d", id, update.UpdateID), "update_id", update.UpdateID)
		go handleUserMessage(ctx, &update, bot, telegramService, logger)
	}
}

// Adds a new webhook route to the mux. Initializes 5 workers to handle all updates (messages).
// Returns a reference to the created bot.
func SetupBot(mux *http.ServeMux, ctx context.Context, logger *applog.Logger, telegramService *TelegramService) (*telego.Bot, error) {
	bot, err := telego.NewBot(config.Cfg.TelegramBotToken, telego.WithLogger(logger))
	if err != nil {
		return nil, err
	}

	_ = bot.SetWebhook(ctx, &telego.SetWebhookParams{
		URL:         config.Cfg.ServerURL + "/bot",
		SecretToken: bot.SecretToken(),
	})

	// Receive information about webhook
	info, _ := bot.GetWebhookInfo(ctx)
	fmt.Printf("Webhook Info: %+v\n", info)

	updates, _ := bot.UpdatesViaWebhook(ctx, telego.WebhookHTTPServeMux(mux, "/bot",
		bot.SecretToken()))

	var workersNum uint16 = 5

	// Start all workers
	for id := range workersNum {
		go Worker(ctx, id, updates, bot, logger, telegramService)
	}

	return bot, nil
}
