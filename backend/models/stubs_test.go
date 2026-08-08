package models

import (
	"context"
	"errors"
	"testing"
)

func TestStubsReturnNotImplemented(t *testing.T) {
	ctx := context.Background()

	if _, err := (StubUserModel{}).GetByID(ctx, 1); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("user model error = %v", err)
	}
	if _, err := (StubTelegramAccountModel{}).GetByTelegramUserID(ctx, 1); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("telegram account model error = %v", err)
	}
	if _, err := (StubEmailVerificationModel{}).GetLatest(ctx, 1, "person@example.com"); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("verification model error = %v", err)
	}
	if err := (StubEmailSender{}).SendVerificationCode(ctx, "person@example.com", "123456"); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("email sender error = %v", err)
	}
}
