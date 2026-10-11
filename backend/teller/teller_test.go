package teller

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"yohanc3/steer/models"
)

type syncUserStoreStub struct{ user models.User }

func (stub syncUserStoreStub) GetUser(context.Context, models.UserID) (models.User, error) {
	return stub.user, nil
}
func (syncUserStoreStub) GetOrCreateUser(context.Context, int64) (models.User, error) {
	return models.User{}, nil
}
func (syncUserStoreStub) ListConnectedUsers(context.Context) ([]models.User, error) { return nil, nil }
func (syncUserStoreStub) SaveTellerConnection(context.Context, models.UserID, models.Account, string, []byte, []byte, string) error {
	return nil
}
func (syncUserStoreStub) MarkBaselineComplete(context.Context, models.UserID, time.Time) error {
	return nil
}

type syncTransactionStoreStub struct{ transactions []models.Transaction }

func (stub *syncTransactionStoreStub) UpsertTransactions(_ context.Context, _ models.UserID, transactions []models.Transaction, _ bool) error {
	stub.transactions = append([]models.Transaction(nil), transactions...)
	return nil
}
func (*syncTransactionStoreStub) ListTransactions(context.Context, models.UserID, time.Time) ([]models.Transaction, error) {
	return nil, nil
}

type syncTellerClientStub struct {
	startDate    string
	endDate      string
	transactions []models.Transaction
}

func (*syncTellerClientStub) ListAccounts(context.Context, string) ([]models.Account, error) {
	return nil, nil
}
func (stub *syncTellerClientStub) ListTransactions(_ context.Context, _, _, startDate, endDate string) ([]models.Transaction, error) {
	stub.startDate = startDate
	stub.endDate = endDate
	return stub.transactions, nil
}
func (*syncTellerClientStub) GetBalances(context.Context, string, string) (models.AccountBalances, error) {
	return models.AccountBalances{}, nil
}

type decryptCipherStub struct{}

func (decryptCipherStub) Encrypt(plaintext []byte) ([]byte, []byte, error) {
	return plaintext, nil, nil
}
func (decryptCipherStub) Decrypt([]byte, []byte) ([]byte, error) { return []byte("token"), nil }

func TestEd25519EnrollmentVerifier(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewEd25519EnrollmentVerifier(base64.StdEncoding.EncodeToString(publicKey))
	if err != nil {
		t.Fatal(err)
	}
	values := []string{"nonce", "token", "usr_1", "enr_1", "sandbox"}
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(strings.Join(values, "."))))
	if err := verifier.Verify(values[0], values[1], values[2], values[3], values[4], []string{signature}); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if err := verifier.Verify("other-nonce", values[1], values[2], values[3], values[4], []string{signature}); err == nil {
		t.Fatal("Verify() accepted a signature for a different nonce")
	}
}

func TestAESGCMDecryptValidatesStoredTokenData(t *testing.T) {
	cipher, err := NewAESGCM([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, nonce, err := cipher.Encrypt([]byte("access-token"))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := cipher.Decrypt(ciphertext, nonce)
	if err != nil || !bytes.Equal(plaintext, []byte("access-token")) {
		t.Fatalf("Decrypt() = %q, %v", plaintext, err)
	}
	if _, err := cipher.Decrypt(ciphertext, nil); err == nil {
		t.Fatal("Decrypt() accepted an invalid nonce")
	}
	if _, err := cipher.Decrypt(nil, nonce); err == nil {
		t.Fatal("Decrypt() accepted an invalid ciphertext")
	}
}

func TestListTransactionsPaginatesDateWindow(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		if request.URL.Query().Get("start_date") != "2026-10-01" || request.URL.Query().Get("end_date") != "2026-10-11" {
			t.Errorf("date query = %s", request.URL.RawQuery)
		}
		if request.URL.Query().Get("count") != "100" {
			t.Errorf("count = %q", request.URL.Query().Get("count"))
		}

		page := make([]transactionResponse, 0, 100)
		if requests == 1 {
			for i := 0; i < 100; i++ {
				page = append(page, transactionResponse{ID: "txn-" + strconv.Itoa(i), AccountID: "acc", Status: "posted"})
			}
		} else {
			if request.URL.Query().Get("from_id") != "txn-99" {
				t.Errorf("from_id = %q", request.URL.Query().Get("from_id"))
			}
			page = append(page, transactionResponse{ID: "txn-100", AccountID: "acc", Status: "posted"})
		}
		if err := json.NewEncoder(w).Encode(page); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	client := &TellerHTTPClient{httpClient: server.Client(), baseURL: server.URL}
	transactions, err := client.ListTransactions(context.Background(), "token", "acc", "2026-10-01", "2026-10-11")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(transactions) != 101 {
		t.Fatalf("requests = %d, transactions = %d", requests, len(transactions))
	}
}

func TestSyncUserUsesOverlapAndStoresOnlyPostedTransactions(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	client := &syncTellerClientStub{transactions: []models.Transaction{
		{ID: "posted", ProviderStatus: "posted"},
		{ID: "pending", ProviderStatus: "pending"},
	}}
	transactions := &syncTransactionStoreStub{}
	service := TellerService{
		TellerClient:      client,
		UserStore:         syncUserStoreStub{user: models.User{ID: "user", TellerAccountID: "acc"}},
		TransactionStore:  transactions,
		AccessTokenCipher: decryptCipherStub{},
		SyncLookbackDays:  10,
		Clock:             func() time.Time { return now },
	}

	if err := service.SyncUser(context.Background(), "user", false); err != nil {
		t.Fatal(err)
	}
	if client.startDate != "2026-10-01" || client.endDate != "2026-10-11" {
		t.Fatalf("window = %s through %s", client.startDate, client.endDate)
	}
	if len(transactions.transactions) != 1 || transactions.transactions[0].ID != "posted" {
		t.Fatalf("stored transactions = %#v", transactions.transactions)
	}
}
