package telegrambot

import (
	"context"
	"fmt"
	"net/http"
	"yohanc3/steer/config"
	applog "yohanc3/steer/logger"

	"github.com/mymmrac/telego"
)

// Handles updates, which are usually messages (hence the function name) 
func handleUserMessage(ctx context.Context, update *telego.Update, bot *telego.Bot) {

	text := "Hi! This is SteerBot"
	bot.SendMessage(ctx, &telego.SendMessageParams{ChatID: update.Message.Chat.ChatID(), Text: text})

}

// Initializes a worker which will handle updates as they come. See handleUserMessage()
func Worker(ctx context.Context, id uint16, updates <-chan telego.Update, bot *telego.Bot, logger *applog.Logger) {
	for update := range updates {
		logger.Debug(fmt.Sprintf("Worker of id %d processing update of id %d", id, update.UpdateID))
		go handleUserMessage(ctx, &update, bot)
	}
}

// Adds a new webhook route to the mux. Initializes 5 workers to handle all updates (messages).
// Returns a reference to the created bot.
func SetupBot(mux *http.ServeMux, ctx context.Context, logger *applog.Logger) (*telego.Bot, error) {
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
		go Worker(ctx, id, updates, bot, logger)
	}

	return bot, nil
}
