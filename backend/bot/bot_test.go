package bot

import (
	"context"
	"errors"
	"testing"

	telegrambot "github.com/go-telegram/bot"
)

type webhookRegistrarStub struct {
	params *telegrambot.SetWebhookParams
	ok     bool
	err    error
}

func (stub *webhookRegistrarStub) SetWebhook(_ context.Context, params *telegrambot.SetWebhookParams) (bool, error) {
	stub.params = params
	return stub.ok, stub.err
}

func TestRegisterWebhookUsesPublicGateway(t *testing.T) {
	stub := &webhookRegistrarStub{ok: true}
	if err := RegisterWebhook(context.Background(), stub, "https://steer.example", "secret"); err != nil {
		t.Fatal(err)
	}
	if stub.params.URL != "https://steer.example/telegram/webhook" {
		t.Fatalf("webhook URL = %s", stub.params.URL)
	}
	if stub.params.SecretToken != "secret" || len(stub.params.AllowedUpdates) != 1 {
		t.Fatalf("webhook params = %#v", stub.params)
	}
}

func TestRegisterWebhookReportsTelegramFailure(t *testing.T) {
	stub := &webhookRegistrarStub{err: errors.New("network unavailable")}
	if err := RegisterWebhook(context.Background(), stub, "https://steer.example", "secret"); err == nil {
		t.Fatal("expected webhook registration error")
	}
}
