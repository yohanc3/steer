package controllers

import (
	"context"
	"errors"
	"testing"
	"time"
	"yohanc3/steer/models"
)

type fakeTelegramIdentityController struct {
	user models.User
	err  error
}

func (controller fakeTelegramIdentityController) Resolve(context.Context, TelegramIdentity) (models.User, error) {
	return controller.user, controller.err
}

type fakeEmailVerificationController struct {
	requestedUserID models.UserID
	requestedEmail  string
	err             error
}

func (controller *fakeEmailVerificationController) Request(_ context.Context, userID models.UserID, email string) error {
	controller.requestedUserID = userID
	controller.requestedEmail = email
	return controller.err
}

func (*fakeEmailVerificationController) Confirm(context.Context, models.UserID, string, time.Time) (models.User, error) {
	return models.User{}, nil
}

func TestTelegramEmailVerificationControllerRequestsVerification(t *testing.T) {
	verification := &fakeEmailVerificationController{}
	controller := TelegramEmailVerificationController{
		Identity:     fakeTelegramIdentityController{user: models.User{ID: 42}},
		Verification: verification,
	}

	err := controller.HandleEmailVerificationRequested(context.Background(), EmailVerificationRequestedEvent{
		Identity: TelegramIdentity{TelegramUserID: 1, TelegramChatID: 2},
		Email:    "person@example.com",
	})
	if err != nil {
		t.Fatalf("HandleEmailVerificationRequested() error = %v", err)
	}
	if verification.requestedUserID != 42 {
		t.Fatalf("requested user ID = %d, want 42", verification.requestedUserID)
	}
	if verification.requestedEmail != "person@example.com" {
		t.Fatalf("requested email = %q, want person@example.com", verification.requestedEmail)
	}
}

func TestTelegramEmailVerificationControllerWrapsIdentityError(t *testing.T) {
	wantErr := errors.New("identity unavailable")
	controller := TelegramEmailVerificationController{
		Identity:     fakeTelegramIdentityController{err: wantErr},
		Verification: &fakeEmailVerificationController{},
	}

	err := controller.HandleEmailVerificationRequested(context.Background(), EmailVerificationRequestedEvent{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("HandleEmailVerificationRequested() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestTelegramEmailVerificationControllerWrapsVerificationError(t *testing.T) {
	wantErr := errors.New("email unavailable")
	controller := TelegramEmailVerificationController{
		Identity:     fakeTelegramIdentityController{user: models.User{ID: 42}},
		Verification: &fakeEmailVerificationController{err: wantErr},
	}

	err := controller.HandleEmailVerificationRequested(context.Background(), EmailVerificationRequestedEvent{Email: "person@example.com"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("HandleEmailVerificationRequested() error = %v, want wrapped %v", err, wantErr)
	}
}
