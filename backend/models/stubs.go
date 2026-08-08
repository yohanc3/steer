package models

import (
	"context"
	"errors"
	"time"
)

var ErrNotImplemented = errors.New("not implemented")

type StubUserModel struct{}

func (StubUserModel) Create(context.Context, string, string) (User, error) {
	return User{}, ErrNotImplemented
}

func (StubUserModel) GetByID(context.Context, UserID) (User, error) {
	return User{}, ErrNotImplemented
}

func (StubUserModel) UpdateName(context.Context, UserID, string) (User, error) {
	return User{}, ErrNotImplemented
}

func (StubUserModel) UpdateEmail(context.Context, UserID, string) (User, error) {
	return User{}, ErrNotImplemented
}

func (StubUserModel) MarkEmailVerified(context.Context, UserID, time.Time) (User, error) {
	return User{}, ErrNotImplemented
}

type StubTelegramAccountModel struct{}

func (StubTelegramAccountModel) GetByTelegramUserID(context.Context, int64) (TelegramAccount, error) {
	return TelegramAccount{}, ErrNotImplemented
}

func (StubTelegramAccountModel) Link(context.Context, TelegramAccount) (TelegramAccount, error) {
	return TelegramAccount{}, ErrNotImplemented
}

type StubEmailVerificationModel struct{}

func (StubEmailVerificationModel) Create(context.Context, EmailVerificationCode) (EmailVerificationCode, error) {
	return EmailVerificationCode{}, ErrNotImplemented
}

func (StubEmailVerificationModel) GetLatest(context.Context, UserID, string) (EmailVerificationCode, error) {
	return EmailVerificationCode{}, ErrNotImplemented
}

func (StubEmailVerificationModel) Consume(context.Context, int64, time.Time) error {
	return ErrNotImplemented
}

type StubEmailSender struct{}

func (StubEmailSender) SendVerificationCode(context.Context, string, string) error {
	return ErrNotImplemented
}

var (
	_ UserModel              = StubUserModel{}
	_ TelegramAccountModel   = StubTelegramAccountModel{}
	_ EmailVerificationModel = StubEmailVerificationModel{}
	_ EmailSender            = StubEmailSender{}
)
