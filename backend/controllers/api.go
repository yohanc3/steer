// Package controllers defines transport-facing backend contracts.
package controllers

import (
	"context"
	"time"
	"yohanc3/steer/models"
)

type TelegramIdentity struct {
	TelegramUserID int64
	TelegramChatID int64
}

type OnboardingStatus struct {
	User          models.User
	Telegram      models.TelegramAccount
	EmailVerified bool
}

type TelegramIdentityController interface {
	Resolve(ctx context.Context, identity TelegramIdentity) (models.User, error)
}

type TelegramOnboardingController interface {
	Start(ctx context.Context, identity TelegramIdentity, name, email string) (OnboardingStatus, error)
	Status(ctx context.Context, identity TelegramIdentity) (OnboardingStatus, error)
}

type EmailVerificationController interface {
	Request(ctx context.Context, userID models.UserID, email string) error
	Confirm(ctx context.Context, userID models.UserID, code string, confirmedAt time.Time) (models.User, error)
}

type EmailVerificationRequestedEvent struct {
	Identity TelegramIdentity
	Email    string
}

type TelegramEventController interface {
	HandleEmailVerificationRequested(ctx context.Context, event EmailVerificationRequestedEvent) error
}
