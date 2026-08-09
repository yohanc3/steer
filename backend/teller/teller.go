package teller

import (
	"context"
	"crypto/aes"
	cryptocipher "crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"yohanc3/steer/models"
)

// TellerClient fetches accounts and transactions from Teller.
type TellerClient interface {
	ListAccounts(ctx context.Context, accessToken string) ([]models.Account, error)
	ListTransactions(ctx context.Context, accessToken, accountID, startDate, endDate string) ([]models.Transaction, error)
}

// TellerHTTPClient is the mutual-TLS implementation of TellerClient.
type TellerHTTPClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewTellerHTTPClient creates the mutual-TLS client used for Teller API calls.
// The PEM certificate and private key must be issued for the Teller application.
func NewTellerHTTPClient(certPEM, keyPEM string) (*TellerHTTPClient, error) {
	certificate, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("load teller certificate: %w", err)
	}
	return &TellerHTTPClient{baseURL: "https://api.teller.io", httpClient: &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}}}}, nil
}

// ListAccounts fetches the accounts authorized by an access token.
// The token must be valid for the configured Teller API environment.
func (tellerClient *TellerHTTPClient) ListAccounts(ctx context.Context, accessToken string) ([]models.Account, error) {
	var response []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		Subtype  string `json:"subtype"`
		Currency string `json:"currency"`
		LastFour string `json:"last_four"`
		Status   string `json:"status"`
	}
	if err := tellerClient.get(ctx, accessToken, "/accounts", &response); err != nil {
		return nil, err
	}
	accounts := make([]models.Account, len(response))
	for i, a := range response {
		accounts[i] = models.Account{ID: a.ID, Name: a.Name, Type: a.Type, Subtype: a.Subtype, Currency: a.Currency, LastFour: a.LastFour, Status: a.Status}
	}
	return accounts, nil
}

// ListTransactions fetches an account's transactions within an inclusive date window.
// Token, account ID, and dates must be valid Teller API request values.
func (tellerClient *TellerHTTPClient) ListTransactions(ctx context.Context, accessToken, accountID, startDate, endDate string) ([]models.Transaction, error) {
	path := "/accounts/" + url.PathEscape(accountID) + "/transactions?start_date=" + url.QueryEscape(startDate) + "&end_date=" + url.QueryEscape(endDate)
	var response []struct {
		ID             string  `json:"id"`
		AccountID      string  `json:"account_id"`
		Amount         string  `json:"amount"`
		Date           string  `json:"date"`
		Description    string  `json:"description"`
		Status         string  `json:"status"`
		Type           string  `json:"type"`
		RunningBalance *string `json:"running_balance"`
		Details        struct {
			ProcessingStatus string  `json:"processing_status"`
			Category         *string `json:"category"`
			Counterparty     struct {
				Name *string `json:"name"`
				Type *string `json:"type"`
			} `json:"counterparty"`
		} `json:"details"`
		Links struct {
			Self    string `json:"self"`
			Account string `json:"account"`
		} `json:"links"`
	}
	if err := tellerClient.get(ctx, accessToken, path, &response); err != nil {
		return nil, err
	}
	transactions := make([]models.Transaction, len(response))
	for i, t := range response {
		transactions[i] = models.Transaction{ID: t.ID, AccountID: t.AccountID, Amount: t.Amount, Date: t.Date, Description: t.Description, Status: t.Status, Type: t.Type, RunningBalance: t.RunningBalance, ProcessingStatus: t.Details.ProcessingStatus, Category: t.Details.Category, CounterpartyName: t.Details.Counterparty.Name, CounterpartyType: t.Details.Counterparty.Type, SelfLink: t.Links.Self, AccountLink: t.Links.Account}
	}
	return transactions, nil
}

// get issues an authenticated Teller GET request and decodes its JSON response.
// The path must be API-relative and destination must accept the endpoint payload.
func (tellerClient *TellerHTTPClient) get(ctx context.Context, accessToken, path string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, tellerClient.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build teller request: %w", err)
	}
	request.SetBasicAuth(accessToken, "")
	response, err := tellerClient.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("perform teller request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("teller rate limited")
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("teller response: %s", response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode teller response: %w", err)
	}
	return nil
}

// AccessTokenCipher encrypts Teller access tokens before persistent storage.
type AccessTokenCipher interface {
	Encrypt([]byte) ([]byte, []byte, error)
	Decrypt([]byte, []byte) ([]byte, error)
}

// TellerService completes verified Teller Connect flows and synchronizes data.
// Its dependencies must be configured with the application's persistent stores.
type TellerService struct {
	TellerClient        TellerClient
	UserStore           models.UserStore
	ConnectSessionStore models.ConnectSessionStore
	TransactionStore    models.TransactionStore
	AccessTokenCipher   AccessTokenCipher
	EnrollmentVerifier  EnrollmentVerifier
	TellerEnvironment   string
	Clock               func() time.Time
}

type EnrollmentVerifier interface {
	Verify(nonce, accessToken, tellerUserID, enrollmentID, environment string, signatures []string) error
}

// Complete validates a browser enrollment and atomically stores its first sync.
// The session token and signed Teller fields must originate from one Connect flow.
func (tellerService TellerService) Complete(ctx context.Context, sessionToken, accessToken, enrollmentID, tellerUserID string, signatures []string) error {
	now := tellerService.now()
	session, err := tellerService.ConnectSessionStore.GetConnectSession(ctx, modelsHash(sessionToken), now)
	if err != nil {
		return fmt.Errorf("get connect session: %w", err)
	}
	if err := tellerService.EnrollmentVerifier.Verify(session.Nonce, accessToken, tellerUserID, enrollmentID, tellerService.TellerEnvironment, signatures); err != nil {
		return fmt.Errorf("verify teller enrollment: %w", err)
	}
	accounts, err := tellerService.TellerClient.ListAccounts(ctx, accessToken)
	if err != nil {
		return fmt.Errorf("list teller accounts: %w", err)
	}
	if len(accounts) != 1 {
		return fmt.Errorf("expected one teller account, got %d", len(accounts))
	}
	ciphertext, nonce, err := tellerService.AccessTokenCipher.Encrypt([]byte(accessToken))
	if err != nil {
		return fmt.Errorf("encrypt teller token: %w", err)
	}
	transactions, err := tellerService.TellerClient.ListTransactions(ctx, accessToken, accounts[0].ID, now.AddDate(0, 0, -30).Format("2006-01-02"), now.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("list initial teller transactions: %w", err)
	}
	return tellerService.ConnectSessionStore.FinalizeConnectSession(ctx, models.ConnectCompletion{TokenHash: modelsHash(sessionToken), TellerAccount: accounts[0], TellerUserID: tellerUserID, EncryptedAccessToken: ciphertext, AccessTokenNonce: nonce, TellerEnvironment: tellerService.TellerEnvironment, BaselineTransactions: transactions, CompletedAt: now})
}

// SyncUser fetches and stores new transactions for a connected Teller user.
// The user must have encrypted credentials and an active Teller account.
func (tellerService TellerService) SyncUser(ctx context.Context, userID models.UserID, baseline bool) error {
	user, err := tellerService.UserStore.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	token, err := tellerService.AccessTokenCipher.Decrypt(user.AccessTokenCiphertext, user.AccessTokenNonce)
	if err != nil {
		return err
	}
	start, err := tellerService.TransactionStore.SyncStartDate(ctx, userID, user.TellerAccountID)
	if err != nil {
		return err
	}
	if start == "" {
		start = tellerService.now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	transactions, err := tellerService.TellerClient.ListTransactions(ctx, string(token), user.TellerAccountID, start, tellerService.now().Format("2006-01-02"))
	if err != nil {
		return err
	}
	if err := tellerService.TransactionStore.UpsertTransactions(ctx, userID, transactions, baseline); err != nil {
		return err
	}
	if baseline {
		return tellerService.UserStore.MarkBaselineComplete(ctx, userID, tellerService.now())
	}
	return nil
}

// now returns the injected clock when testing or the current UTC time in production.
// An injected clock must return a value that can be converted to UTC.
func (tellerService TellerService) now() time.Time {
	if tellerService.Clock != nil {
		return tellerService.Clock().UTC()
	}
	return time.Now().UTC()
}

// modelsHash creates the session-token digest shared by Teller service and storage.
// It must receive the original browser token, not an already hashed value.
func modelsHash(token string) []byte { sum := sha256.Sum256([]byte(token)); return sum[:] }

// Ed25519EnrollmentVerifier verifies the signed payload returned by Teller
// Connect when initialized with a server-generated nonce.
type Ed25519EnrollmentVerifier struct{ PublicKey ed25519.PublicKey }

// NewEd25519EnrollmentVerifier builds a verifier from Teller's configured public key.
// The key must be PEM or a raw Ed25519 key encoded as base64 or hexadecimal.
func NewEd25519EnrollmentVerifier(encodedKey string) (*Ed25519EnrollmentVerifier, error) {
	key, err := decodeEd25519PublicKey(encodedKey)
	if err != nil {
		return nil, err
	}
	return &Ed25519EnrollmentVerifier{PublicKey: key}, nil
}

// Verify accepts one signature matching the exact Teller Connect enrollment payload.
// Nonce, token, IDs, environment, and signatures must all be supplied by Connect.
func (verifier Ed25519EnrollmentVerifier) Verify(nonce, accessToken, tellerUserID, enrollmentID, environment string, signatures []string) error {
	if nonce == "" || accessToken == "" || tellerUserID == "" || enrollmentID == "" || environment == "" {
		return errors.New("incomplete signed enrollment")
	}
	message := []byte(strings.Join([]string{nonce, accessToken, tellerUserID, enrollmentID, environment}, "."))
	for _, encodedSignature := range signatures {
		signature, err := decodeBase64OrHex(encodedSignature)
		if err == nil && ed25519.Verify(verifier.PublicKey, message, signature) {
			return nil
		}
	}
	return errors.New("invalid teller enrollment signature")
}

// decodeEd25519PublicKey parses Teller's PEM or encoded raw Ed25519 public key.
// The supplied key must decode to exactly one Ed25519 public key.
func decodeEd25519PublicKey(value string) (ed25519.PublicKey, error) {
	if block, _ := pem.Decode([]byte(value)); block != nil {
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse Teller signing key: %w", err)
		}
		publicKey, ok := key.(ed25519.PublicKey)
		if !ok {
			return nil, errors.New("Teller signing key is not Ed25519")
		}
		return publicKey, nil
	}
	key, err := decodeBase64OrHex(value)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("invalid Teller Ed25519 signing key")
	}
	return ed25519.PublicKey(key), nil
}

// decodeBase64OrHex decodes the configured key or signature encodings accepted by Teller.
// The value must be nonempty, correctly padded when required, and valid in one format.
func decodeBase64OrHex(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	for _, decoder := range []func(string) ([]byte, error){base64.StdEncoding.DecodeString, base64.RawStdEncoding.DecodeString, base64.URLEncoding.DecodeString, base64.RawURLEncoding.DecodeString, hex.DecodeString} {
		decoded, err := decoder(value)
		if err == nil {
			return decoded, nil
		}
	}
	return nil, errors.New("invalid encoded value")
}

type AESGCM struct{ block cryptocipher.Block }

// NewAESGCM constructs the authenticated cipher for stored Teller access tokens.
// Key material must be an AES-valid length and should come from runtime configuration.
func NewAESGCM(key []byte) (*AESGCM, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create token cipher: %w", err)
	}
	return &AESGCM{block: block}, nil
}

// Encrypt seals plaintext with a new random nonce using AES-GCM.
// The caller must store both returned ciphertext and nonce together.
func (cipher *AESGCM) Encrypt(plaintext []byte) ([]byte, []byte, error) {
	gcm, err := cipher.NewGCM()
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nonce, nil
}

// Decrypt authenticates and opens ciphertext using its matching AES-GCM nonce.
// Ciphertext and nonce must have been returned by Encrypt under the same key.
func (cipher *AESGCM) Decrypt(ciphertext, nonce []byte) ([]byte, error) {
	gcm, err := cipher.NewGCM()
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt teller token: %w", err)
	}
	return plaintext, nil
}

// NewGCM exposes a standard AEAD instance for the configured AES block cipher.
// It is used by Encrypt and Decrypt and may fail only for invalid cipher setup.
func (cipher *AESGCM) NewGCM() (cryptocipher.AEAD, error) { return cryptocipher.NewGCM(cipher.block) }
