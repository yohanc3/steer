package controllers

import (
	"context"
	"time"
	"yohanc3/steer/models"
)

type StubTelegramIdentityController struct{}

func (StubTelegramIdentityController) Resolve(context.Context, TelegramIdentity) (models.User, error) {
	return models.User{}, models.ErrNotImplemented
}

type StubTelegramOnboardingController struct{}

func (StubTelegramOnboardingController) Start(context.Context, TelegramIdentity, string, string) (OnboardingStatus, error) {
	return OnboardingStatus{}, models.ErrNotImplemented
}

func (StubTelegramOnboardingController) Status(context.Context, TelegramIdentity) (OnboardingStatus, error) {
	return OnboardingStatus{}, models.ErrNotImplemented
}

type StubEmailVerificationController struct{}

func (StubEmailVerificationController) Request(context.Context, models.UserID, string) error {
	return models.ErrNotImplemented
}

func (StubEmailVerificationController) Confirm(context.Context, models.UserID, string, time.Time) (models.User, error) {
	return models.User{}, models.ErrNotImplemented
}

type StubTelegramEventController struct{}

func (StubTelegramEventController) HandleEmailVerificationRequested(context.Context, EmailVerificationRequestedEvent) error {
	return models.ErrNotImplemented
}

var (
	_ TelegramIdentityController   = StubTelegramIdentityController{}
	_ TelegramOnboardingController = StubTelegramOnboardingController{}
	_ EmailVerificationController  = StubEmailVerificationController{}
	_ TelegramEventController      = StubTelegramEventController{}
)
