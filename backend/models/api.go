// Package models defines backend contracts used by controllers.
package models

import (
	"context"
	"time"
)

type UserID int64

type User struct {
	ID              UserID
	Name            string
	Email           string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type TelegramAccount struct {
	TelegramUserID int64
	TelegramChatID int64
	UserID         UserID
	CreatedAt      time.Time
}

type EmailVerificationCode struct {
	ID         int64
	UserID     UserID
	Email      string
	CodeHash   []byte
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	Attempts   int
	CreatedAt  time.Time
}

type UserModel interface {
	Create(ctx context.Context, name, email string) (User, error)
	GetByID(ctx context.Context, userID UserID) (User, error)
	UpdateName(ctx context.Context, userID UserID, name string) (User, error)
	UpdateEmail(ctx context.Context, userID UserID, email string) (User, error)
	MarkEmailVerified(ctx context.Context, userID UserID, verifiedAt time.Time) (User, error)
}

type TelegramAccountModel interface {
	GetByTelegramUserID(ctx context.Context, telegramUserID int64) (TelegramAccount, error)
	Link(ctx context.Context, account TelegramAccount) (TelegramAccount, error)
}

type EmailVerificationModel interface {
	Create(ctx context.Context, code EmailVerificationCode) (EmailVerificationCode, error)
	GetLatest(ctx context.Context, userID UserID, email string) (EmailVerificationCode, error)
	Consume(ctx context.Context, codeID int64, consumedAt time.Time) error
}

type EmailSender interface {
	SendVerificationCode(ctx context.Context, email, code string) error
}
