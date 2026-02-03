package telegrambot

import (
	"context"
	"fmt"
	"net/http"
	"yohanc3/steer/config"

	"github.com/mymmrac/telego"
)

func SetupBot(mux *http.ServeMux, ctx context.Context) error {
	bot, err := telego.NewBot(config.Cfg.TelegramBotToken, telego.WithDefaultDebugLogger())
	if err != nil {
		return err
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
	go func() {

		for update := range updates {
			fmt.Printf("update: %+v\n", update)
		}
	}()
	return nil
}
