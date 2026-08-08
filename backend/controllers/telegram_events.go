package controllers

import (
	"context"
	"fmt"
)

// TelegramEmailVerificationController translates a Telegram verification event
// into a request to the email-verification controller.
type TelegramEmailVerificationController struct {
	Identity     TelegramIdentityController
	Verification EmailVerificationController
}

func (controller TelegramEmailVerificationController) HandleEmailVerificationRequested(ctx context.Context, event EmailVerificationRequestedEvent) error {
	user, err := controller.Identity.Resolve(ctx, event.Identity)
	if err != nil {
		return fmt.Errorf("resolve Telegram identity: %w", err)
	}

	if err := controller.Verification.Request(ctx, user.ID, event.Email); err != nil {
		return fmt.Errorf("request email verification: %w", err)
	}

	return nil
}

var _ TelegramEventController = TelegramEmailVerificationController{}
