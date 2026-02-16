package telegrambot

import (
	"context"
	"fmt"
	"net/http"
	"yohanc3/steer/config"
	applog "yohanc3/steer/logger"

	"github.com/mymmrac/telego"
)

func handleUserMessage(update *telego.Update) {

	fmt.Println("New update id:", update.UpdateID, "update messager name:", update.Message.From.FirstName, "update text:", update.Message.Text)

}

func Worker(id uint16, updates <-chan telego.Update, logger *applog.Logger) {
	for update := range updates {
		logger.Debug(fmt.Sprintf("Worker of id %d processing update of id %d", id, update.UpdateID))
		go handleUserMessage(&update)
	}
}

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

	for id := range workersNum {
		go Worker(id, updates, logger)
	}

	return bot, nil
}
