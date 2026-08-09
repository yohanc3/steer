package bot

import (
	"context"
	"errors"
	"strings"
	"testing"

	telegrambot "github.com/go-telegram/bot"

	"yohanc3/steer/models"
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

func TestFormatTransactionMessagesAddsReadableAmountDateAndBalance(t *testing.T) {
	balance := "125.50"
	messages := formatTransactionMessages(
		[]models.Transaction{{Amount: "-12.30", Date: "2026-08-06", Description: "Coffee", Status: "posted", RunningBalance: &balance}},
		false,
		&balance,
	)
	for _, expected := range []string{"<b>-$12.30</b>", "Thu, Aug 6th 2026", "<b>Available balance</b>", "$125.50"} {
		if !strings.Contains(messages[0], expected) {
			t.Fatalf("message missing %q: %s", expected, messages[0])
		}
	}
}

func TestFormatTransactionMessagesUsesLiveBalanceForEmptyHistory(t *testing.T) {
	balance := "125.50"
	messages := formatTransactionMessages(nil, false, &balance)
	if len(messages) != 1 || !strings.Contains(messages[0], "Available balance") {
		t.Fatalf("messages = %#v", messages)
	}
}
