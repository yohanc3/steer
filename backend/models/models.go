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

// DeliveryStatus tracks whether a posted transaction still needs notification delivery.
type DeliveryStatus string

const (
	DeliveryStatusSettled   DeliveryStatus = "settled"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
)

// CategorizationStatus tracks whether a posted transaction has completed classification.
type CategorizationStatus string

const (
	CategorizationStatusPending       CategorizationStatus = "pending"
	CategorizationStatusCategorized   CategorizationStatus = "categorized"
	CategorizationStatusUncategorized CategorizationStatus = "uncategorized"
)

// CategorizationSource records how an assignment was made for audit and policy decisions.
type CategorizationSource string

const (
	CategorizationSourceJev          CategorizationSource = "jev"
	CategorizationSourceJevUncertain CategorizationSource = "jev_uncertain"
	CategorizationSourceJevError     CategorizationSource = "jev_error"
	CategorizationSourceMerchantRule CategorizationSource = "merchant_rule"
	CategorizationSourceUser         CategorizationSource = "user"
	CategorizationSourceNoCategories CategorizationSource = "no_categories"
)

// TransactionCategory is one user-owned classification option.
type TransactionCategory struct {
	ID          string
	UserID      UserID
	Name        string
	Description string
}

// Transaction is Teller transaction data normalized for local persistence.
type Transaction struct {
	ID                       string
	AccountID                string
	Amount                   string
	Date                     string
	Description              string
	ProviderStatus           string
	DeliveryStatus           DeliveryStatus
	Type                     string
	RunningBalance           *string
	ProcessingStatus         string
	Category                 *string
	UserCategoryID           *string
	UserCategoryName         *string
	CategorizationStatus     CategorizationStatus
	CategorizationSource     *CategorizationSource
	CategorizationConfidence *float64
	CategorizationModel      *string
	CounterpartyName         *string
	CounterpartyType         *string
	SelfLink                 string
	AccountLink              string
}

// CategorizationCandidate is a pending transaction plus any remembered exact-name rule.
type CategorizationCandidate struct {
	Transaction          Transaction
	UserID               UserID
	RememberedCategoryID *string
	CreatedAt            int64
}

// CategoryDecision is a TypeSafe Choice translated back to application category IDs.
type CategoryDecision struct {
	CategoryID    *string
	Confidence    float64
	Model         string
	Probabilities map[string]float64
}

// CategorizationResult is the durable outcome of one categorization attempt.
type CategorizationResult struct {
	TransactionID string
	CategoryID    *string
	Status        CategorizationStatus
	Source        CategorizationSource
	Confidence    *float64
	Model         *string
	Probabilities map[string]float64
}

// TransactionDelivery is one locally settled transaction and its Telegram destination.
// CreatedAt and Transaction.ID form the stable cursor used while draining a delivery cycle.
type TransactionDelivery struct {
	Transaction            Transaction
	TelegramConversationID int64
	CreatedAt              int64
}

// AccountBalances is Teller's live balance view for a connected account.
type AccountBalances struct {
	Available *string
	Ledger    *string
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

// TransactionStore persists and reads Teller transactions.
type TransactionStore interface {
	UpsertTransactions(ctx context.Context, userID UserID, transactions []Transaction, baseline bool) error
	ListTransactions(ctx context.Context, userID UserID, since time.Time) ([]Transaction, error)
}
