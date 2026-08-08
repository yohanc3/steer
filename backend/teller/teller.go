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

type Client interface {
	ListAccounts(ctx context.Context, accessToken string) ([]models.Account, error)
	ListTransactions(ctx context.Context, accessToken, accountID, startDate, endDate string) ([]models.Transaction, error)
}

type HTTPClient struct {
	client  *http.Client
	baseURL string
}

func NewHTTPClient(certPEM, keyPEM string) (*HTTPClient, error) {
	certificate, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("load teller certificate: %w", err)
	}
	return &HTTPClient{baseURL: "https://api.teller.io", client: &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}}}}, nil
}

func (client *HTTPClient) ListAccounts(ctx context.Context, token string) ([]models.Account, error) {
	var response []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		Subtype  string `json:"subtype"`
		Currency string `json:"currency"`
		LastFour string `json:"last_four"`
		Status   string `json:"status"`
	}
	if err := client.get(ctx, token, "/accounts", &response); err != nil {
		return nil, err
	}
	accounts := make([]models.Account, len(response))
	for i, a := range response {
		accounts[i] = models.Account{ID: a.ID, Name: a.Name, Type: a.Type, Subtype: a.Subtype, Currency: a.Currency, LastFour: a.LastFour, Status: a.Status}
	}
	return accounts, nil
}

func (client *HTTPClient) ListTransactions(ctx context.Context, token, accountID, startDate, endDate string) ([]models.Transaction, error) {
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
	if err := client.get(ctx, token, path, &response); err != nil {
		return nil, err
	}
	transactions := make([]models.Transaction, len(response))
	for i, t := range response {
		transactions[i] = models.Transaction{ID: t.ID, AccountID: t.AccountID, Amount: t.Amount, Date: t.Date, Description: t.Description, Status: t.Status, Type: t.Type, RunningBalance: t.RunningBalance, ProcessingStatus: t.Details.ProcessingStatus, Category: t.Details.Category, CounterpartyName: t.Details.Counterparty.Name, CounterpartyType: t.Details.Counterparty.Type, SelfLink: t.Links.Self, AccountLink: t.Links.Account}
	}
	return transactions, nil
}

func (client *HTTPClient) get(ctx context.Context, token, path string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build teller request: %w", err)
	}
	request.SetBasicAuth(token, "")
	response, err := client.client.Do(request)
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

type Cipher interface {
	Encrypt([]byte) ([]byte, []byte, error)
	Decrypt([]byte, []byte) ([]byte, error)
}

// TellerService completes verified Teller Connect flows and synchronizes data.
// Its dependencies must be configured with the application's persistent stores.
type TellerService struct {
	Client       Client
	Users        models.UserStore
	Sessions     models.ConnectSessionStore
	Transactions models.TransactionStore
	Cipher       Cipher
	Verifier     EnrollmentVerifier
	Environment  string
	Now          func() time.Time
}

type EnrollmentVerifier interface {
	Verify(nonce, accessToken, tellerUserID, enrollmentID, environment string, signatures []string) error
}

func (service TellerService) Complete(ctx context.Context, sessionToken, accessToken, enrollmentID, tellerUserID string, signatures []string) error {
	now := service.now()
	session, err := service.Sessions.GetConnectSession(ctx, modelsHash(sessionToken), now)
	if err != nil {
		return fmt.Errorf("get connect session: %w", err)
	}
	if err := service.Verifier.Verify(session.Nonce, accessToken, tellerUserID, enrollmentID, service.Environment, signatures); err != nil {
		return fmt.Errorf("verify teller enrollment: %w", err)
	}
	accounts, err := service.Client.ListAccounts(ctx, accessToken)
	if err != nil {
		return fmt.Errorf("list teller accounts: %w", err)
	}
	if len(accounts) != 1 {
		return fmt.Errorf("expected one teller account, got %d", len(accounts))
	}
	ciphertext, nonce, err := service.Cipher.Encrypt([]byte(accessToken))
	if err != nil {
		return fmt.Errorf("encrypt teller token: %w", err)
	}
	transactions, err := service.Client.ListTransactions(ctx, accessToken, accounts[0].ID, now.AddDate(0, 0, -30).Format("2006-01-02"), now.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("list initial teller transactions: %w", err)
	}
	return service.Sessions.FinalizeConnectSession(ctx, models.ConnectCompletion{TokenHash: modelsHash(sessionToken), Account: accounts[0], TellerUserID: tellerUserID, AccessToken: ciphertext, AccessTokenNonce: nonce, Environment: service.Environment, Transactions: transactions, CompletedAt: now})
}
func (service TellerService) SyncUser(ctx context.Context, userID models.UserID, baseline bool) error {
	user, err := service.Users.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	token, err := service.Cipher.Decrypt(user.AccessTokenCiphertext, user.AccessTokenNonce)
	if err != nil {
		return err
	}
	start, err := service.Transactions.SyncStartDate(ctx, userID, user.TellerAccountID)
	if err != nil {
		return err
	}
	if start == "" {
		start = service.now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	transactions, err := service.Client.ListTransactions(ctx, string(token), user.TellerAccountID, start, service.now().Format("2006-01-02"))
	if err != nil {
		return err
	}
	if err := service.Transactions.UpsertTransactions(ctx, userID, transactions, baseline); err != nil {
		return err
	}
	if baseline {
		return service.Users.MarkBaselineComplete(ctx, userID, service.now())
	}
	return nil
}
func (service TellerService) now() time.Time {
	if service.Now != nil {
		return service.Now().UTC()
	}
	return time.Now().UTC()
}
func modelsHash(token string) []byte { sum := sha256.Sum256([]byte(token)); return sum[:] }

// Ed25519EnrollmentVerifier verifies the signed payload returned by Teller
// Connect when initialized with a server-generated nonce.
type Ed25519EnrollmentVerifier struct{ PublicKey ed25519.PublicKey }

func NewEd25519EnrollmentVerifier(encodedKey string) (*Ed25519EnrollmentVerifier, error) {
	key, err := decodeEd25519PublicKey(encodedKey)
	if err != nil {
		return nil, err
	}
	return &Ed25519EnrollmentVerifier{PublicKey: key}, nil
}

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

func NewAESGCM(key []byte) (*AESGCM, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create token cipher: %w", err)
	}
	return &AESGCM{block: block}, nil
}
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
func (cipher *AESGCM) NewGCM() (cryptocipher.AEAD, error) { return cryptocipher.NewGCM(cipher.block) }
