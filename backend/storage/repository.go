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

type Repository struct{ Database *sql.DB }

// GetOrCreateUser returns the local user for a Telegram conversation.
// It looks up the conversation first and creates a UUID-backed user if absent.
func (repository Repository) GetOrCreateUser(ctx context.Context, conversationID int64) (models.User, error) {
	// Prefer the existing conversation mapping to keep user creation idempotent.
	var id string
	err := repository.Database.QueryRowContext(ctx, `SELECT id FROM users WHERE telegram_conversation_id = ?`, conversationID).Scan(&id)
	if err == nil {
		return repository.GetUser(ctx, models.UserID(id))
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return models.User{}, fmt.Errorf("find user: %w", err)
	}
	// Create a stable internal ID only after confirming the conversation is new.
	id = uuid.NewString()
	if _, err := repository.Database.ExecContext(ctx, `INSERT INTO users(id, telegram_conversation_id) VALUES (?, ?)`, id, conversationID); err != nil {
		return models.User{}, fmt.Errorf("create user: %w", err)
	}
	return repository.GetUser(ctx, models.UserID(id))
}

// GetUser loads local and Teller connection state for a user ID.
// The ID must refer to a persisted user; absent users return ErrNotFound.
func (repository Repository) GetUser(ctx context.Context, userID models.UserID) (models.User, error) {
	// Scan nullable connection state separately so unconnected users remain valid.
	var user models.User
	var baseline sql.NullInt64
	var accountID, tellerUserID, environment sql.NullString
	err := repository.Database.QueryRowContext(ctx, `SELECT id, telegram_conversation_id, teller_account_id, teller_user_id, teller_access_token_ciphertext, teller_access_token_nonce, teller_environment, baseline_completed_at FROM users WHERE id = ?`, userID).Scan(&user.ID, &user.TelegramConversationID, &accountID, &tellerUserID, &user.AccessTokenCiphertext, &user.AccessTokenNonce, &environment, &baseline)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("get user: %w", err)
	}
	// Convert nullable database fields into the model's zero-value representation.
	user.TellerAccountID = accountID.String
	user.TellerUserID = tellerUserID.String
	user.TellerEnvironment = environment.String
	if baseline.Valid {
		value := time.Unix(baseline.Int64, 0).UTC()
		user.BaselineCompletedAt = &value
	}
	return user, nil
}

// ListConnectedUsers returns users whose encrypted Teller credentials are stored.
// It closes the ID query before loading each user because SQLite has one connection.
func (repository Repository) ListConnectedUsers(ctx context.Context) ([]models.User, error) {
	// Fetch IDs first so the rows cursor is closed before reusing SQLite's connection.
	rows, err := repository.Database.QueryContext(ctx, `SELECT id FROM users WHERE teller_account_id IS NOT NULL AND teller_access_token_ciphertext IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("list connected users: %w", err)
	}
	var ids []models.UserID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan user ID: %w", err)
		}
		ids = append(ids, models.UserID(id))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate connected user IDs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close connected user IDs: %w", err)
	}

	// Hydrate each connection only after the initial query has released the database.
	users := make([]models.User, 0, len(ids))
	for _, id := range ids {
		user, err := repository.GetUser(ctx, id)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

// SaveTellerConnection replaces a user's active Teller account and credentials.
// The account and user must be valid Teller values and userID must already exist.
func (repository Repository) SaveTellerConnection(ctx context.Context, userID models.UserID, account models.Account, tellerUserID string, ciphertext, nonce []byte, environment string) error {
	// Replacing a connection resets its baseline so the selected account is reclassified.
	result, err := repository.Database.ExecContext(ctx, `UPDATE users SET teller_account_id=?, teller_user_id=?, teller_access_token_ciphertext=?, teller_access_token_nonce=?, teller_environment=?, baseline_completed_at=NULL, updated_at=unixepoch() WHERE id=?`, account.ID, tellerUserID, ciphertext, nonce, environment, userID)
	if err != nil {
		return fmt.Errorf("save teller connection: %w", err)
	}
	// Distinguish an absent user from a successful update with no silent fallback.
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("teller connection rows: %w", err)
	}
	if changed != 1 {
		return ErrNotFound
	}
	return nil
}

// MarkBaselineComplete records when initial transaction classification finishes.
// The timestamp is persisted in UTC seconds for an existing user.
func (repository Repository) MarkBaselineComplete(ctx context.Context, userID models.UserID, at time.Time) error {
	// Store UTC seconds so baseline state uses the same SQLite representation as sessions.
	_, err := repository.Database.ExecContext(ctx, `UPDATE users SET baseline_completed_at=?, updated_at=unixepoch() WHERE id=?`, at.Unix(), userID)
	if err != nil {
		return fmt.Errorf("mark baseline complete: %w", err)
	}
	return nil
}

// CreateConnectSession stores a one-time, expiring Teller Connect session.
// The caller must hash the browser token and provide a user-owned nonce.
func (repository Repository) CreateConnectSession(ctx context.Context, session models.ConnectSession) error {
	// Persist only the token hash; the bearer token is never written to SQLite.
	_, err := repository.Database.ExecContext(ctx, `INSERT INTO teller_connect_sessions(token_hash, user_id, nonce, expires_at) VALUES (?, ?, ?, ?)`, session.TokenHash, session.UserID, session.Nonce, session.ExpiresAt.Unix())
	if err != nil {
		return fmt.Errorf("create connect session: %w", err)
	}
	return nil
}

// GetConnectSession returns a live, unconsumed session for a hashed browser token.
// Expired, consumed, or unknown tokens deliberately return ErrNotFound.
func (repository Repository) GetConnectSession(ctx context.Context, tokenHash []byte, now time.Time) (models.ConnectSession, error) {
	// Load the session before applying expiry and single-use checks in application time.
	var session models.ConnectSession
	var expires, consumed int64
	var consumedNull sql.NullInt64
	err := repository.Database.QueryRowContext(ctx, `SELECT token_hash, user_id, nonce, expires_at, consumed_at FROM teller_connect_sessions WHERE token_hash=?`, tokenHash).Scan(&session.TokenHash, &session.UserID, &session.Nonce, &expires, &consumedNull)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ConnectSession{}, ErrNotFound
	}
	if err != nil {
		return models.ConnectSession{}, fmt.Errorf("get connect session: %w", err)
	}
	// Reconstruct timestamps before rejecting consumed or expired sessions uniformly.
	session.ExpiresAt = time.Unix(expires, 0)
	if consumedNull.Valid {
		consumed = consumedNull.Int64
		value := time.Unix(consumed, 0)
		session.ConsumedAt = &value
	}
	if session.ConsumedAt != nil || !session.ExpiresAt.After(now) {
		return models.ConnectSession{}, ErrNotFound
	}
	return session, nil
}

// FinalizeConnectSession atomically records a completed Teller connection,
// its initial transaction baseline, and one-time session consumption.
// Completion must contain a validated token hash, encrypted credentials, and data.
func (repository Repository) FinalizeConnectSession(ctx context.Context, completion models.ConnectCompletion) error {
	// Keep session consumption, connection storage, and baseline import all-or-nothing.
	tx, err := repository.Database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin connect completion: %w", err)
	}
	defer tx.Rollback()

	// Atomically claim the still-live token before looking up its owning user.
	var userID models.UserID
	result, err := tx.ExecContext(ctx, `UPDATE teller_connect_sessions SET consumed_at=? WHERE token_hash=? AND consumed_at IS NULL AND expires_at>?`, completion.CompletedAt.Unix(), completion.TokenHash, completion.CompletedAt.Unix())
	if err != nil {
		return fmt.Errorf("consume connect session: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return ErrNotFound
	}
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM teller_connect_sessions WHERE token_hash=?`, completion.TokenHash).Scan(&userID); err != nil {
		return fmt.Errorf("get connect session user: %w", err)
	}
	// Store the validated credentials and baseline timestamp for the session owner.
	result, err = tx.ExecContext(ctx, `UPDATE users SET teller_account_id=?, teller_user_id=?, teller_access_token_ciphertext=?, teller_access_token_nonce=?, teller_environment=?, baseline_completed_at=?, updated_at=unixepoch() WHERE id=?`, completion.TellerAccount.ID, completion.TellerUserID, completion.EncryptedAccessToken, completion.AccessTokenNonce, completion.TellerEnvironment, completion.CompletedAt.Unix(), userID)
	if err != nil {
		return fmt.Errorf("save teller connection: %w", err)
	}
	changed, err = result.RowsAffected()
	if err != nil || changed != 1 {
		return ErrNotFound
	}
	// Classify the initial window as baseline before committing every connection change.
	if err := upsertTransactions(ctx, tx, userID, completion.BaselineTransactions, true); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit connect completion: %w", err)
	}
	return nil
}

// UpsertTransactions writes Teller transactions outside a connect finalization.
// The user must exist; baseline controls how the supplied records are classified.
func (repository Repository) UpsertTransactions(ctx context.Context, userID models.UserID, transactions []models.Transaction, baseline bool) error {
	return upsertTransactions(ctx, repository.Database, userID, transactions, baseline)
}

// ListTransactions returns a user's locally stored transactions from the requested date onward.
func (repository Repository) ListTransactions(ctx context.Context, userID models.UserID, since time.Time) ([]models.Transaction, error) {
	rows, err := repository.Database.QueryContext(ctx, `SELECT t.transaction_id, t.account_id, t.amount, t.transaction_date, t.description, t.provider_status, t.delivery_status, t.transaction_type, t.running_balance, t.processing_status, t.category, t.user_category_id, c.name, t.categorization_status, t.categorization_source, t.categorization_confidence, t.categorization_model, t.counterparty_name, t.counterparty_type, t.self_link, t.account_link FROM transactions AS t LEFT JOIN transaction_categories AS c ON c.category_id=t.user_category_id WHERE t.user_id=? AND t.transaction_date>=? ORDER BY t.transaction_date DESC`, userID, since.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()
	var transactions []models.Transaction
	for rows.Next() {
		var transaction models.Transaction
		if err := rows.Scan(&transaction.ID, &transaction.AccountID, &transaction.Amount, &transaction.Date, &transaction.Description, &transaction.ProviderStatus, &transaction.DeliveryStatus, &transaction.Type, &transaction.RunningBalance, &transaction.ProcessingStatus, &transaction.Category, &transaction.UserCategoryID, &transaction.UserCategoryName, &transaction.CategorizationStatus, &transaction.CategorizationSource, &transaction.CategorizationConfidence, &transaction.CategorizationModel, &transaction.CounterpartyName, &transaction.CounterpartyType, &transaction.SelfLink, &transaction.AccountLink); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transactions: %w", err)
	}
	return transactions, nil
}

// ListSettledTransactionDeliveries returns the next non-baseline notification batch.
// The composite cursor lets one cycle advance past failures without retrying them immediately.
func (repository Repository) ListSettledTransactionDeliveries(ctx context.Context, afterCreatedAt int64, afterTransactionID string, limit int) ([]models.TransactionDelivery, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("delivery limit must be greater than zero")
	}
	rows, err := repository.Database.QueryContext(ctx, `
SELECT
    t.transaction_id,
    t.account_id,
    t.amount,
    t.transaction_date,
    t.description,
    t.provider_status,
    t.delivery_status,
    t.transaction_type,
    t.running_balance,
    t.processing_status,
    t.category,
    t.user_category_id,
    c.name,
    t.categorization_status,
    t.categorization_source,
    t.categorization_confidence,
    t.categorization_model,
    t.counterparty_name,
    t.counterparty_type,
    t.self_link,
    t.account_link,
    u.telegram_conversation_id,
    t.created_at
FROM transactions AS t
JOIN users AS u ON u.id = t.user_id
LEFT JOIN transaction_categories AS c ON c.category_id = t.user_category_id
WHERE t.baseline = 0
  AND t.provider_status = 'posted'
  AND t.delivery_status = 'settled'
  AND t.categorization_status <> 'pending'
  AND (t.created_at > ? OR (t.created_at = ? AND t.transaction_id > ?))
ORDER BY t.created_at, t.transaction_id
LIMIT ?`, afterCreatedAt, afterCreatedAt, afterTransactionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list settled transaction deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []models.TransactionDelivery
	for rows.Next() {
		var delivery models.TransactionDelivery
		transaction := &delivery.Transaction
		if err := rows.Scan(&transaction.ID, &transaction.AccountID, &transaction.Amount, &transaction.Date, &transaction.Description, &transaction.ProviderStatus, &transaction.DeliveryStatus, &transaction.Type, &transaction.RunningBalance, &transaction.ProcessingStatus, &transaction.Category, &transaction.UserCategoryID, &transaction.UserCategoryName, &transaction.CategorizationStatus, &transaction.CategorizationSource, &transaction.CategorizationConfidence, &transaction.CategorizationModel, &transaction.CounterpartyName, &transaction.CounterpartyType, &transaction.SelfLink, &transaction.AccountLink, &delivery.TelegramConversationID, &delivery.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan settled transaction delivery: %w", err)
		}
		deliveries = append(deliveries, delivery)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settled transaction deliveries: %w", err)
	}
	return deliveries, nil
}

// MarkTransactionDelivered completes one notification only if it is still settled.
func (repository Repository) MarkTransactionDelivered(ctx context.Context, transactionID string) error {
	result, err := repository.Database.ExecContext(ctx, `UPDATE transactions SET delivery_status='delivered', updated_at=unixepoch() WHERE transaction_id=? AND delivery_status='settled'`, transactionID)
	if err != nil {
		return fmt.Errorf("mark transaction delivered: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delivered transaction rows: %w", err)
	}
	if changed != 1 {
		return ErrNotFound
	}
	return nil
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

const upsertTransactionSQL = `
INSERT INTO transactions (
    transaction_id,
    user_id,
    account_id,
    amount,
    transaction_date,
    description,
    provider_status,
    transaction_type,
    running_balance,
    processing_status,
    category,
    counterparty_name,
    counterparty_type,
    self_link,
    account_link,
    baseline
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(transaction_id) DO UPDATE SET
    user_id = excluded.user_id,
    account_id = excluded.account_id,
    amount = excluded.amount,
    transaction_date = excluded.transaction_date,
    description = excluded.description,
    provider_status = excluded.provider_status,
    transaction_type = excluded.transaction_type,
    running_balance = excluded.running_balance,
    processing_status = excluded.processing_status,
    category = excluded.category,
    counterparty_name = excluded.counterparty_name,
    counterparty_type = excluded.counterparty_type,
    self_link = excluded.self_link,
    account_link = excluded.account_link,
    baseline = MAX(transactions.baseline, excluded.baseline),
    updated_at = unixepoch()
`

// upsertTransactions executes transaction writes with a database or SQL transaction.
// The executor must remain valid for every supplied transaction in the batch.
func upsertTransactions(ctx context.Context, executor sqlExecutor, userID models.UserID, transactions []models.Transaction, baseline bool) error {
	// Apply each Teller record through the caller's database or transaction executor.
	for _, transaction := range transactions {
		_, err := executor.ExecContext(ctx, upsertTransactionSQL, transaction.ID, userID, transaction.AccountID, transaction.Amount, transaction.Date, transaction.Description, transaction.ProviderStatus, transaction.Type, transaction.RunningBalance, transaction.ProcessingStatus, transaction.Category, transaction.CounterpartyName, transaction.CounterpartyType, transaction.SelfLink, transaction.AccountLink, boolToInt(baseline))
		if err != nil {
			return fmt.Errorf("upsert transaction: %w", err)
		}
	}
	return nil
}

// HashToken produces the fixed-size database representation of a browser token.
// Callers must retain only this digest, never the original bearer token.
func HashToken(token string) []byte { sum := sha256.Sum256([]byte(token)); return sum[:] }

// boolToInt converts SQLite boolean values to their integer representation.
// It is used only for columns declared as INTEGER flags.
func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
