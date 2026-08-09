package models

import (
	"context"
	"time"
)

// UserID identifies one local user across stores and services.
type UserID string

// User holds Telegram identity and the active Teller connection state.
type User struct {
	ID                     UserID
	TelegramConversationID int64
	TellerAccountID        string
	TellerUserID           string
	AccessTokenCiphertext  []byte
	AccessTokenNonce       []byte
	TellerEnvironment      string
	BaselineCompletedAt    *time.Time
}

// ConnectSession binds a browser completion to a one-time Telegram-initiated request.
type ConnectSession struct {
	TokenHash  []byte
	UserID     UserID
	Nonce      string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

// ConnectCompletion is the fully validated result of a Teller Connect session.
// Persisting it consumes the session, saves the connection, and imports the
// initial baseline as one database transaction.
type ConnectCompletion struct {
	TokenHash            []byte
	TellerAccount        Account
	TellerUserID         string
	EncryptedAccessToken []byte
	AccessTokenNonce     []byte
	TellerEnvironment    string
	BaselineTransactions []Transaction
	CompletedAt          time.Time
}

// Account is the Teller account selected for synchronization.
type Account struct {
	ID       string
	Name     string
	Type     string
	Subtype  string
	Currency string
	LastFour string
	Status   string
}

// Transaction is Teller transaction data normalized for local persistence.
type Transaction struct {
	ID               string
	AccountID        string
	Amount           string
	Date             string
	Description      string
	Status           string
	Type             string
	RunningBalance   *string
	ProcessingStatus string
	Category         *string
	CounterpartyName *string
	CounterpartyType *string
	SelfLink         string
	AccountLink      string
}

// UserStore persists local users and their Teller connection metadata.
type UserStore interface {
	GetOrCreateUser(ctx context.Context, conversationID int64) (User, error)
	GetUser(ctx context.Context, userID UserID) (User, error)
	ListConnectedUsers(ctx context.Context) ([]User, error)
	SaveTellerConnection(ctx context.Context, userID UserID, account Account, tellerUserID string, ciphertext, nonce []byte, environment string) error
	MarkBaselineComplete(ctx context.Context, userID UserID, at time.Time) error
}

// ConnectSessionStore persists one-time browser completion sessions.
type ConnectSessionStore interface {
	CreateConnectSession(ctx context.Context, session ConnectSession) error
	GetConnectSession(ctx context.Context, tokenHash []byte, now time.Time) (ConnectSession, error)
	FinalizeConnectSession(ctx context.Context, completion ConnectCompletion) error
}

// TransactionStore persists Teller transactions and computes account sync cursors.
type TransactionStore interface {
	UpsertTransactions(ctx context.Context, userID UserID, transactions []Transaction, baseline bool) error
	SyncStartDate(ctx context.Context, userID UserID, accountID string) (string, error)
	ListTransactions(ctx context.Context, userID UserID, since time.Time) ([]Transaction, error)
}
