package bot

import (
	"context"
	"errors"
	"strings"
	"testing"

	telegrambot "github.com/go-telegram/bot"

	"yohanc3/steer/models"
)

type webhookDeleterStub struct {
	params *telegrambot.DeleteWebhookParams
	ok     bool
	err    error
}

func (stub *webhookDeleterStub) DeleteWebhook(_ context.Context, params *telegrambot.DeleteWebhookParams) (bool, error) {
	stub.params = params
	return stub.ok, stub.err
}

func TestPrepareLongPollingRemovesExistingWebhook(t *testing.T) {
	stub := &webhookDeleterStub{ok: true}
	if err := PrepareLongPolling(context.Background(), stub); err != nil {
		t.Fatal(err)
	}
	if stub.params == nil || stub.params.DropPendingUpdates {
		t.Fatalf("delete webhook params = %#v", stub.params)
	}
}

func TestPrepareLongPollingReportsTelegramFailure(t *testing.T) {
	stub := &webhookDeleterStub{err: errors.New("network unavailable")}
	if err := PrepareLongPolling(context.Background(), stub); err == nil {
		t.Fatal("expected webhook deletion error")
	}
}

func TestFormatTransactionMessagesAddsReadableAmountDateAndBalance(t *testing.T) {
	balance := "125.50"
	messages := formatTransactionMessages(
		[]models.Transaction{{Amount: "-12.30", Date: "2026-08-06", Description: "Coffee", ProviderStatus: "posted", RunningBalance: &balance}},
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

func TestHasTellerConnection(t *testing.T) {
	if hasTellerConnection(models.User{}) {
		t.Fatal("empty user reported as connected")
	}
	if hasTellerConnection(models.User{TellerAccountID: "acc_1", AccessTokenCiphertext: []byte("ciphertext")}) {
		t.Fatal("user without a nonce reported as connected")
	}
	if !hasTellerConnection(models.User{TellerAccountID: "acc_1", AccessTokenCiphertext: []byte("ciphertext"), AccessTokenNonce: []byte("nonce")}) {
		t.Fatal("complete Teller credentials reported as disconnected")
	}
}
