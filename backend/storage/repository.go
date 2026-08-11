package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"yohanc3/steer/budget"
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
	rows, err := repository.Database.QueryContext(ctx, `SELECT transaction_id, account_id, amount, transaction_date, description, status, transaction_type, running_balance, processing_status, category, counterparty_name, counterparty_type, self_link, account_link FROM transactions WHERE user_id=? AND transaction_date>=? ORDER BY transaction_date DESC`, userID, since.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()
	var transactions []models.Transaction
	for rows.Next() {
		var transaction models.Transaction
		if err := rows.Scan(&transaction.ID, &transaction.AccountID, &transaction.Amount, &transaction.Date, &transaction.Description, &transaction.Status, &transaction.Type, &transaction.RunningBalance, &transaction.ProcessingStatus, &transaction.Category, &transaction.CounterpartyName, &transaction.CounterpartyType, &transaction.SelfLink, &transaction.AccountLink); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transactions: %w", err)
	}
	return transactions, nil
}

// GetPrototypeBudget loads a browser-scoped prototype budget when one exists.
func (repository Repository) GetPrototypeBudget(ctx context.Context, clientID string) (*budget.Budget, error) {
	var storedCategories string
	prototypeBudget := budget.Budget{}
	var updatedAt int64
	err := repository.Database.QueryRowContext(ctx, `SELECT monthly_total_cents, categories_json, version, updated_at FROM prototype_budgets WHERE client_id = ?`, clientID).Scan(&prototypeBudget.MonthlyTotalCents, &storedCategories, &prototypeBudget.Version, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, budget.ErrNoBudget
	}
	if err != nil {
		return nil, fmt.Errorf("query prototype budget: %w", err)
	}
	if err := json.Unmarshal([]byte(storedCategories), &prototypeBudget.Categories); err != nil {
		return nil, fmt.Errorf("decode prototype categories: %w", err)
	}
	prototypeBudget.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &prototypeBudget, nil
}

// SavePrototypeBudget replaces the flexible category allocation for one browser session.
func (repository Repository) SavePrototypeBudget(ctx context.Context, clientID string, value budget.Budget) error {
	categories, err := json.Marshal(value.Categories)
	if err != nil {
		return fmt.Errorf("encode prototype categories: %w", err)
	}
	_, err = repository.Database.ExecContext(ctx, `INSERT INTO prototype_budgets(client_id, monthly_total_cents, categories_json, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(client_id) DO UPDATE SET monthly_total_cents=excluded.monthly_total_cents, categories_json=excluded.categories_json, version=excluded.version, updated_at=excluded.updated_at`, clientID, value.MonthlyTotalCents, categories, value.Version, value.UpdatedAt.Unix(), value.UpdatedAt.Unix())
	if err != nil {
		return fmt.Errorf("write prototype budget: %w", err)
	}
	return nil
}

// DeletePrototypeBudget removes the budget and its cached mappings but retains activity history.
func (repository Repository) DeletePrototypeBudget(ctx context.Context, clientID string) error {
	tx, err := repository.Database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete prototype budget: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM prototype_budget_mappings WHERE client_id = ?`, clientID); err != nil {
		return fmt.Errorf("delete prototype mappings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE prototype_budget_transactions SET budget_version = 0, budget_category = NULL, classification_status = 'unmatched' WHERE client_id = ?`, clientID); err != nil {
		return fmt.Errorf("clear prototype classifications: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM prototype_budgets WHERE client_id = ?`, clientID); err != nil {
		return fmt.Errorf("delete prototype budget: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete prototype budget: %w", err)
	}
	return nil
}

// DeletePrototype clears all data created by one browser-scoped prototype session.
func (repository Repository) DeletePrototype(ctx context.Context, clientID string) error {
	tx, err := repository.Database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reset prototype: %w", err)
	}
	defer tx.Rollback()
	for _, query := range []string{
		`DELETE FROM prototype_budget_mappings WHERE client_id = ?`,
		`DELETE FROM prototype_budget_transactions WHERE client_id = ?`,
		`DELETE FROM prototype_budgets WHERE client_id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, query, clientID); err != nil {
			return fmt.Errorf("reset prototype: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reset prototype: %w", err)
	}
	return nil
}

// ListPrototypeTransactions returns recent prototype activity with its classification state.
func (repository Repository) ListPrototypeTransactions(ctx context.Context, clientID string) ([]budget.Transaction, error) {
	rows, err := repository.Database.QueryContext(ctx, `SELECT id, merchant, provider_category, amount_cents, occurred_at, budget_version, budget_category, classification_status, classification_source FROM prototype_budget_transactions WHERE client_id = ? ORDER BY created_at DESC`, clientID)
	if err != nil {
		return nil, fmt.Errorf("query prototype transactions: %w", err)
	}
	defer rows.Close()
	transactions := []budget.Transaction{}
	for rows.Next() {
		var transaction budget.Transaction
		var occurredAt int64
		if err := rows.Scan(&transaction.ID, &transaction.Merchant, &transaction.ProviderCategory, &transaction.AmountCents, &occurredAt, &transaction.BudgetVersion, &transaction.BudgetCategory, &transaction.ClassificationStatus, &transaction.ClassificationSource); err != nil {
			return nil, fmt.Errorf("scan prototype transaction: %w", err)
		}
		transaction.OccurredAt = time.Unix(occurredAt, 0).UTC()
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prototype transactions: %w", err)
	}
	return transactions, nil
}

// GetPrototypeMapping retrieves a cache entry scoped to one budget version.
func (repository Repository) GetPrototypeMapping(ctx context.Context, clientID string, budgetVersion int64, signature string) (string, error) {
	var category string
	err := repository.Database.QueryRowContext(ctx, `SELECT budget_category FROM prototype_budget_mappings WHERE client_id = ? AND budget_version = ? AND source_signature = ?`, clientID, budgetVersion, signature).Scan(&category)
	if errors.Is(err, sql.ErrNoRows) {
		return "", budget.ErrNoMapping
	}
	if err != nil {
		return "", fmt.Errorf("query prototype mapping: %w", err)
	}
	return category, nil
}

// SavePrototypeMapping records a reusable category decision for the current budget version.
func (repository Repository) SavePrototypeMapping(ctx context.Context, clientID string, budgetVersion int64, signature, category string) error {
	_, err := repository.Database.ExecContext(ctx, `INSERT INTO prototype_budget_mappings(client_id, budget_version, source_signature, budget_category) VALUES (?, ?, ?, ?) ON CONFLICT(client_id, budget_version, source_signature) DO UPDATE SET budget_category=excluded.budget_category`, clientID, budgetVersion, signature, category)
	if err != nil {
		return fmt.Errorf("write prototype mapping: %w", err)
	}
	return nil
}

// CreatePrototypeTransaction stores the activity regardless of whether it was classified.
func (repository Repository) CreatePrototypeTransaction(ctx context.Context, clientID string, transaction budget.Transaction) error {
	_, err := repository.Database.ExecContext(ctx, `INSERT INTO prototype_budget_transactions(id, client_id, merchant, provider_category, amount_cents, occurred_at, budget_version, budget_category, classification_status, classification_source) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, transaction.ID, clientID, transaction.Merchant, transaction.ProviderCategory, transaction.AmountCents, transaction.OccurredAt.Unix(), transaction.BudgetVersion, transaction.BudgetCategory, transaction.ClassificationStatus, transaction.ClassificationSource)
	if err != nil {
		return fmt.Errorf("write prototype transaction: %w", err)
	}
	return nil
}

// ResolvePrototypeTransaction records a user-approved category for an unmatched activity.
func (repository Repository) ResolvePrototypeTransaction(ctx context.Context, clientID, transactionID, category string) error {
	result, err := repository.Database.ExecContext(ctx, `UPDATE prototype_budget_transactions SET budget_category = ?, classification_status = 'matched', classification_source = 'user' WHERE id = ? AND client_id = ? AND classification_status = 'unmatched'`, category, transactionID, clientID)
	if err != nil {
		return fmt.Errorf("update prototype transaction: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("prototype transaction rows: %w", err)
	}
	if changed != 1 {
		return ErrNotFound
	}
	return nil
}

// GetTelegramBudget loads the real budget associated with one Telegram user.
func (repository Repository) GetTelegramBudget(ctx context.Context, userID models.UserID) (*budget.Budget, error) {
	var categories string
	value := budget.Budget{}
	var updatedAt int64
	err := repository.Database.QueryRowContext(ctx, `SELECT monthly_total_cents, categories_json, version, updated_at FROM telegram_budgets WHERE user_id = ?`, userID).Scan(&value.MonthlyTotalCents, &categories, &value.Version, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, budget.ErrNoBudget
	}
	if err != nil {
		return nil, fmt.Errorf("query Telegram budget: %w", err)
	}
	if err := json.Unmarshal([]byte(categories), &value.Categories); err != nil {
		return nil, fmt.Errorf("decode Telegram categories: %w", err)
	}
	value.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &value, nil
}

// SaveTelegramBudget atomically replaces a user's complete validated budget.
func (repository Repository) SaveTelegramBudget(ctx context.Context, userID models.UserID, value budget.Budget) error {
	categories, err := json.Marshal(value.Categories)
	if err != nil {
		return fmt.Errorf("encode Telegram categories: %w", err)
	}
	_, err = repository.Database.ExecContext(ctx, `INSERT INTO telegram_budgets(user_id, monthly_total_cents, categories_json, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(user_id) DO UPDATE SET monthly_total_cents=excluded.monthly_total_cents, categories_json=excluded.categories_json, version=excluded.version, updated_at=excluded.updated_at`, userID, value.MonthlyTotalCents, categories, value.Version, value.UpdatedAt.Unix(), value.UpdatedAt.Unix())
	if err != nil {
		return fmt.Errorf("save Telegram budget: %w", err)
	}
	return nil
}

// SaveTelegramBudgetAction persists an action receipt or returns the prior receipt for a webhook retry.
func (repository Repository) SaveTelegramBudgetAction(ctx context.Context, receipt budget.TelegramActionReceipt) (budget.TelegramActionReceipt, error) {
	_, err := repository.Database.ExecContext(ctx, `INSERT INTO telegram_budget_actions(id, user_id, telegram_message_id, action_index, action_type, payload_json, status, result_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(user_id, telegram_message_id, action_index) DO NOTHING`, receipt.ID, receipt.UserID, receipt.TelegramMessageID, receipt.ActionIndex, receipt.ActionType, receipt.Payload, receipt.Status, receipt.Result)
	if err != nil {
		return budget.TelegramActionReceipt{}, fmt.Errorf("save Telegram action: %w", err)
	}
	return receipt, nil
}

// ListTelegramBudgetActions returns prior receipts for idempotent Telegram webhook deliveries.
func (repository Repository) ListTelegramBudgetActions(ctx context.Context, userID models.UserID, messageID int64) ([]budget.TelegramActionReceipt, error) {
	rows, err := repository.Database.QueryContext(ctx, `SELECT id, telegram_message_id, action_index, action_type, payload_json, status, result_json FROM telegram_budget_actions WHERE user_id = ? AND telegram_message_id = ? ORDER BY action_index`, userID, messageID)
	if err != nil {
		return nil, fmt.Errorf("list Telegram actions: %w", err)
	}
	defer rows.Close()
	var receipts []budget.TelegramActionReceipt
	for rows.Next() {
		var receipt budget.TelegramActionReceipt
		if err := rows.Scan(&receipt.ID, &receipt.TelegramMessageID, &receipt.ActionIndex, &receipt.ActionType, &receipt.Payload, &receipt.Status, &receipt.Result); err != nil {
			return nil, fmt.Errorf("scan Telegram action: %w", err)
		}
		receipt.UserID = userID
		receipts = append(receipts, receipt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Telegram actions: %w", err)
	}
	return receipts, nil
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
    status,
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
    status = excluded.status,
    transaction_type = excluded.transaction_type,
    running_balance = excluded.running_balance,
    processing_status = excluded.processing_status,
    category = excluded.category,
    counterparty_name = excluded.counterparty_name,
    counterparty_type = excluded.counterparty_type,
    self_link = excluded.self_link,
    account_link = excluded.account_link,
    baseline = excluded.baseline,
    updated_at = unixepoch()
`

// upsertTransactions executes transaction writes with a database or SQL transaction.
// The executor must remain valid for every supplied transaction in the batch.
func upsertTransactions(ctx context.Context, executor sqlExecutor, userID models.UserID, transactions []models.Transaction, baseline bool) error {
	// Apply each Teller record through the caller's database or transaction executor.
	for _, transaction := range transactions {
		_, err := executor.ExecContext(ctx, upsertTransactionSQL, transaction.ID, userID, transaction.AccountID, transaction.Amount, transaction.Date, transaction.Description, transaction.Status, transaction.Type, transaction.RunningBalance, transaction.ProcessingStatus, transaction.Category, transaction.CounterpartyName, transaction.CounterpartyType, transaction.SelfLink, transaction.AccountLink, boolToInt(baseline))
		if err != nil {
			return fmt.Errorf("upsert transaction: %w", err)
		}
	}
	return nil
}

// SyncStartDate returns the earliest pending or latest completed account date.
// The account ID scopes reconnects so prior accounts cannot advance its cursor.
func (repository Repository) SyncStartDate(ctx context.Context, userID models.UserID, accountID string) (string, error) {
	// Pending records take priority so polling keeps revisiting unsettled transactions.
	var date string
	err := repository.Database.QueryRowContext(ctx, `SELECT transaction_date FROM transactions WHERE user_id=? AND account_id=? AND processing_status='pending' ORDER BY transaction_date ASC LIMIT 1`, userID, accountID).Scan(&date)
	// Otherwise continue from the newest completed transaction for this account only.
	if errors.Is(err, sql.ErrNoRows) {
		err = repository.Database.QueryRowContext(ctx, `SELECT transaction_date FROM transactions WHERE user_id=? AND account_id=? AND processing_status='complete' ORDER BY transaction_date DESC LIMIT 1`, userID, accountID).Scan(&date)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("sync start date: %w", err)
	}
	return date, nil
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
