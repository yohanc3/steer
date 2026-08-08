package controllers

import (
	"context"
	"errors"
	"testing"
	"yohanc3/steer/models"
)

func TestStubsReturnNotImplemented(t *testing.T) {
	ctx := context.Background()
	identity := TelegramIdentity{TelegramUserID: 1, TelegramChatID: 2}

	if _, err := (StubTelegramIdentityController{}).Resolve(ctx, identity); !errors.Is(err, models.ErrNotImplemented) {
		t.Fatalf("identity controller error = %v", err)
	}
	if _, err := (StubTelegramOnboardingController{}).Start(ctx, identity, "Name", "person@example.com"); !errors.Is(err, models.ErrNotImplemented) {
		t.Fatalf("onboarding controller error = %v", err)
	}
	if err := (StubEmailVerificationController{}).Request(ctx, 1, "person@example.com"); !errors.Is(err, models.ErrNotImplemented) {
		t.Fatalf("verification controller error = %v", err)
	}
	if err := (StubTelegramEventController{}).HandleEmailVerificationRequested(ctx, EmailVerificationRequestedEvent{Identity: identity, Email: "person@example.com"}); !errors.Is(err, models.ErrNotImplemented) {
		t.Fatalf("telegram event controller error = %v", err)
	}
}
