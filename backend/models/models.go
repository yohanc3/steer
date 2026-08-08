package models

import (
	"context"
	"time"
)

type UserID string

type User struct {
	ID                     UserID
	TelegramConversationID int64
	TellerAccountID        string
	TellerEnrollmentID     string
	TellerUserID           string
	AccessTokenCiphertext  []byte
	AccessTokenNonce       []byte
	TellerEnvironment      string
	BaselineCompletedAt    *time.Time
}

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
	TokenHash        []byte
	Account          Account
	EnrollmentID     string
	TellerUserID     string
	AccessToken      []byte
	AccessTokenNonce []byte
	Environment      string
	Transactions     []Transaction
	CompletedAt      time.Time
}

type Account struct {
	ID           string
	EnrollmentID string
	Name         string
	Type         string
	Subtype      string
	Currency     string
	LastFour     string
	Status       string
}

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

type UserStore interface {
	GetOrCreateUser(ctx context.Context, conversationID int64) (User, error)
	GetUser(ctx context.Context, userID UserID) (User, error)
	ListConnectedUsers(ctx context.Context) ([]User, error)
	SaveTellerConnection(ctx context.Context, userID UserID, account Account, enrollmentID, tellerUserID string, ciphertext, nonce []byte, environment string) error
	MarkBaselineComplete(ctx context.Context, userID UserID, at time.Time) error
}

type ConnectSessionStore interface {
	CreateConnectSession(ctx context.Context, session ConnectSession) error
	GetConnectSession(ctx context.Context, tokenHash []byte, now time.Time) (ConnectSession, error)
	FinalizeConnectSession(ctx context.Context, completion ConnectCompletion) error
}

type TransactionStore interface {
	UpsertTransactions(ctx context.Context, userID UserID, transactions []Transaction, baseline bool) error
	SyncStartDate(ctx context.Context, userID UserID, accountID string) (string, error)
}
