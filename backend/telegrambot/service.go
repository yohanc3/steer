package telegrambot

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	applog "yohanc3/steer/logger"
	customStrings "yohanc3/steer/strings"

	"github.com/bytedance/gopkg/util/logger"
)

type Code struct {
	Code      string `json:"code"`
	ExpiresAt int64  `json:"expires_at"`
}

type OTP struct {
	ID        int    `json:"id"`
	UserID    string `json:"user_id"`
	Code      string `json:"code"`
	ExpiresAt int    `json:"expires_at"`
	CreatedAt int    `json:"created_at"`
}

type TelegramServiceInterface interface {
	GetConnectionCode(user_id int) (*Code, error)
	DeepLinkAccount(code string) error
}

type TelegramService struct {
	Logger *applog.Logger
	DB     *sql.DB
}

func (tel *TelegramService) DeepLinkAccount(ctx context.Context, code string, conversation_id int64) error {

	tel.Logger.Debug("deep linking account", "code", code)

	tx, err := tel.DB.BeginTx(ctx, nil)
	defer tx.Rollback()

	if err != nil {
		return fmt.Errorf("error when starting db transaction: %w", err)
	}

	rows := tx.QueryRowContext(ctx, `
		SELECT * FROM otp
		WHERE code = ?
		LIMIT 1
	`, code)

	var otp *OTP = &OTP{}

	if err := rows.Scan(&otp.ID, &otp.UserID, &otp.Code, &otp.ExpiresAt, &otp.CreatedAt); err != nil {
		return fmt.Errorf("error scanning OTP: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE user 
		SET conversation_id = ?
		WHERE id = ?
	`, conversation_id, otp.UserID)

	if err != nil {
		return fmt.Errorf("error when assigning user a conversation id: %w", err)
	}

	err = tx.Commit()

	if err != nil {
		return fmt.Errorf("error when commiting after setting user's conversation id: %w", err)
	}

	logger.Debug("Successfully set conversation id to user.", "user_id", otp.UserID, "conversation_id", conversation_id)

	return nil

}

// Retrieves a valid connection code.
// A connection code is allows users to connect their accounts to a telegram conversation
func (tel *TelegramService) GetConnectionCode(ctx context.Context, user_id string) (*Code, error) {
	// Start transactions
	tx, err := tel.DB.BeginTx(ctx, nil)
	defer tx.Rollback()

	if err != nil {
		return nil, fmt.Errorf("error when starting db transaction for user: %s %w", user_id, err)
	}

	// row := tx.QueryContext(ctx, `
	// 	SELECT 
	//
	// 	`)

	// Select a valid Code
	row := tx.QueryRowContext(ctx, `
		SELECT code, expires_at FROM otp
		WHERE user_id = ?
		AND expires_at >= (unixepoch()) 
		LIMIT 1`, user_id)

	var otp *Code = &Code{}

	err = row.Scan(&otp.Code, &otp.ExpiresAt)

	// Return otp if scan is successful
	if err == nil {

		tx.Commit()
		return otp, nil

		// Upsert new valid otp to the database if the current one is invalid
	} else if err != nil && err == sql.ErrNoRows {

		code := customStrings.RandomString(10)
		expires_at := time.Now().Add(time.Hour * 24).UnixMilli()

		// Insert otp into the database. If current user already has one,
		// just replace the code and expiration date
		_, err := tx.ExecContext(ctx, `
			INSERT INTO otp (user_id, code, expires_at) 
			VALUES (?, ?, ?) 
			ON CONFLICT (user_id)
			DO UPDATE SET 
				code = EXCLUDED.code,
				expires_at = EXCLUDED.expires_at
			`, user_id, code, expires_at)

		if err != nil {
			return nil, fmt.Errorf("error when inserting new otp into database. user_id: %s %w", user_id, err)
		}

		tx.Commit()

		return &Code{Code: code, ExpiresAt: expires_at}, nil

	} else {
		return nil, fmt.Errorf("error when scanning db code and expiration date for user: %s %w", user_id, err)
	}

}
