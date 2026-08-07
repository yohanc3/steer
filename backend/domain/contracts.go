// Package domain defines transport- and storage-independent application contracts.
package domain

import (
	"context"
	"time"
)

type UserID int64

type BudgetLimit struct {
	Category    string `json:"category"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
}

type BudgetSnapshot struct {
	Category       string
	Currency       string
	LimitCents     int64
	UsedCents      int64
	RemainingCents int64
	PercentUsed    float64
	Budgeted       bool
}

type Account struct {
	ID                   string
	EnrollmentID         string
	Name                 string
	InstitutionID        string
	InstitutionName      string
	Type                 string
	Subtype              string
	Currency             string
	LastFour             string
	Status               string
	SupportsTransactions bool
}

type Transaction struct {
	ID               string
	AccountID        string
	AmountCents      int64
	Date             time.Time
	Description      string
	Category         string
	CounterpartyName string
	CounterpartyType string
	Status           string
	ProcessingStatus string
	Type             string
}

type TransactionChange struct {
	Current  Transaction
	Previous *Transaction
	Snapshot BudgetSnapshot
}

type BudgetParser interface {
	Parse(ctx context.Context, text string) ([]BudgetLimit, error)
}

type BudgetService interface {
	ReplaceLimits(ctx context.Context, userID UserID, limits []BudgetLimit) error
	Snapshot(ctx context.Context, userID UserID, category string, month time.Time) (BudgetSnapshot, error)
}

type TellerClient interface {
	ListAccounts(ctx context.Context, accessToken string) ([]Account, error)
	ListTransactions(ctx context.Context, accessToken, accountID string, startDate, endDate *time.Time) ([]Transaction, error)
}

type TransactionSyncer interface {
	SyncEnrollment(ctx context.Context, enrollmentID string) error
}

type Notifier interface {
	EnqueueTransactionChange(ctx context.Context, userID UserID, change TransactionChange) error
	EnqueueSystemMessage(ctx context.Context, userID UserID, message string) error
}

type EnrollmentService interface {
	CreateConnectURL(ctx context.Context, userID UserID) (string, error)
}
