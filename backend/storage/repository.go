package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"yohanc3/steer/models"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

type Repository struct{ DB *sql.DB }

func (repository Repository) GetOrCreateUser(ctx context.Context, conversationID int64) (models.User, error) {
	var id string
	err := repository.DB.QueryRowContext(ctx, `SELECT id FROM users WHERE telegram_conversation_id = ?`, conversationID).Scan(&id)
	if err == nil {
		return repository.GetUser(ctx, models.UserID(id))
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return models.User{}, fmt.Errorf("find user: %w", err)
	}
	id = uuid.NewString()
	if _, err := repository.DB.ExecContext(ctx, `INSERT INTO users(id, telegram_conversation_id) VALUES (?, ?)`, id, conversationID); err != nil {
		return models.User{}, fmt.Errorf("create user: %w", err)
	}
	return repository.GetUser(ctx, models.UserID(id))
}

func (repository Repository) GetUser(ctx context.Context, userID models.UserID) (models.User, error) {
	var user models.User
	var baseline sql.NullInt64
	err := repository.DB.QueryRowContext(ctx, `SELECT id, telegram_conversation_id, COALESCE(teller_account_id,''), COALESCE(teller_enrollment_id,''), COALESCE(teller_user_id,''), teller_access_token_ciphertext, teller_access_token_nonce, COALESCE(teller_environment,''), baseline_completed_at FROM users WHERE id = ?`, userID).Scan(&user.ID, &user.TelegramConversationID, &user.TellerAccountID, &user.TellerEnrollmentID, &user.TellerUserID, &user.AccessTokenCiphertext, &user.AccessTokenNonce, &user.TellerEnvironment, &baseline)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("get user: %w", err)
	}
	if baseline.Valid {
		value := time.Unix(baseline.Int64, 0).UTC()
		user.BaselineCompletedAt = &value
	}
	return user, nil
}

func (repository Repository) ListConnectedUsers(ctx context.Context) ([]models.User, error) {
	rows, err := repository.DB.QueryContext(ctx, `SELECT id FROM users WHERE teller_account_id IS NOT NULL AND teller_access_token_ciphertext IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("list connected users: %w", err)
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan user ID: %w", err)
		}
		user, err := repository.GetUser(ctx, models.UserID(id))
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (repository Repository) SaveTellerConnection(ctx context.Context, userID models.UserID, account models.Account, enrollmentID, tellerUserID string, ciphertext, nonce []byte, environment string) error {
	result, err := repository.DB.ExecContext(ctx, `UPDATE users SET teller_account_id=?, teller_enrollment_id=?, teller_user_id=?, teller_access_token_ciphertext=?, teller_access_token_nonce=?, teller_environment=?, baseline_completed_at=NULL, updated_at=unixepoch() WHERE id=?`, account.ID, enrollmentID, tellerUserID, ciphertext, nonce, environment, userID)
	if err != nil {
		return fmt.Errorf("save teller connection: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("teller connection rows: %w", err)
	}
	if changed != 1 {
		return ErrNotFound
	}
	return nil
}

func (repository Repository) MarkBaselineComplete(ctx context.Context, userID models.UserID, at time.Time) error {
	_, err := repository.DB.ExecContext(ctx, `UPDATE users SET baseline_completed_at=?, updated_at=unixepoch() WHERE id=?`, at.Unix(), userID)
	if err != nil {
		return fmt.Errorf("mark baseline complete: %w", err)
	}
	return nil
}

func (repository Repository) CreateConnectSession(ctx context.Context, session models.ConnectSession) error {
	_, err := repository.DB.ExecContext(ctx, `INSERT INTO teller_connect_sessions(token_hash, user_id, nonce, expires_at) VALUES (?, ?, ?, ?)`, session.TokenHash, session.UserID, session.Nonce, session.ExpiresAt.Unix())
	if err != nil {
		return fmt.Errorf("create connect session: %w", err)
	}
	return nil
}

func (repository Repository) ConsumeConnectSession(ctx context.Context, tokenHash []byte, now time.Time) (models.ConnectSession, error) {
	var session models.ConnectSession
	var expires, consumed int64
	var consumedNull sql.NullInt64
	err := repository.DB.QueryRowContext(ctx, `SELECT token_hash, user_id, nonce, expires_at, consumed_at FROM teller_connect_sessions WHERE token_hash=?`, tokenHash).Scan(&session.TokenHash, &session.UserID, &session.Nonce, &expires, &consumedNull)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ConnectSession{}, ErrNotFound
	}
	if err != nil {
		return models.ConnectSession{}, fmt.Errorf("get connect session: %w", err)
	}
	session.ExpiresAt = time.Unix(expires, 0)
	if consumedNull.Valid {
		consumed = consumedNull.Int64
		value := time.Unix(consumed, 0)
		session.ConsumedAt = &value
	}
	if session.ConsumedAt != nil || !session.ExpiresAt.After(now) {
		return models.ConnectSession{}, ErrNotFound
	}
	result, err := repository.DB.ExecContext(ctx, `UPDATE teller_connect_sessions SET consumed_at=? WHERE token_hash=? AND consumed_at IS NULL`, now.Unix(), tokenHash)
	if err != nil {
		return models.ConnectSession{}, fmt.Errorf("consume connect session: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return models.ConnectSession{}, ErrNotFound
	}
	return session, nil
}

func (repository Repository) UpsertTransactions(ctx context.Context, userID models.UserID, transactions []models.Transaction, baseline bool) error {
	for _, transaction := range transactions {
		_, err := repository.DB.ExecContext(ctx, `INSERT INTO transactions(transaction_id,user_id,account_id,amount,transaction_date,description,status,transaction_type,running_balance,processing_status,category,counterparty_name,counterparty_type,self_link,account_link,baseline) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(transaction_id) DO UPDATE SET user_id=excluded.user_id,account_id=excluded.account_id,amount=excluded.amount,transaction_date=excluded.transaction_date,description=excluded.description,status=excluded.status,transaction_type=excluded.transaction_type,running_balance=excluded.running_balance,processing_status=excluded.processing_status,category=excluded.category,counterparty_name=excluded.counterparty_name,counterparty_type=excluded.counterparty_type,self_link=excluded.self_link,account_link=excluded.account_link,updated_at=unixepoch()`, transaction.ID, userID, transaction.AccountID, transaction.Amount, transaction.Date, transaction.Description, transaction.Status, transaction.Type, transaction.RunningBalance, transaction.ProcessingStatus, transaction.Category, transaction.CounterpartyName, transaction.CounterpartyType, transaction.SelfLink, transaction.AccountLink, boolToInt(baseline))
		if err != nil {
			return fmt.Errorf("upsert transaction: %w", err)
		}
	}
	return nil
}

func (repository Repository) SyncStartDate(ctx context.Context, userID models.UserID) (string, error) {
	var date string
	err := repository.DB.QueryRowContext(ctx, `SELECT transaction_date FROM transactions WHERE user_id=? AND processing_status='pending' ORDER BY transaction_date ASC LIMIT 1`, userID).Scan(&date)
	if errors.Is(err, sql.ErrNoRows) {
		err = repository.DB.QueryRowContext(ctx, `SELECT transaction_date FROM transactions WHERE user_id=? AND processing_status='complete' ORDER BY transaction_date DESC LIMIT 1`, userID).Scan(&date)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("sync start date: %w", err)
	}
	return date, nil
}

func HashToken(token string) []byte { sum := sha256.Sum256([]byte(token)); return sum[:] }
func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
