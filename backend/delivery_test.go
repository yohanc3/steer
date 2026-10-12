package main

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"yohanc3/steer/models"
)

type deliveryStoreStub struct {
	mu         sync.Mutex
	deliveries []models.TransactionDelivery
	marked     []string
	limits     []int
}

func (stub *deliveryStoreStub) ListSettledTransactionDeliveries(_ context.Context, afterCreatedAt int64, afterTransactionID string, limit int) ([]models.TransactionDelivery, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.limits = append(stub.limits, limit)
	var result []models.TransactionDelivery
	for _, delivery := range stub.deliveries {
		if delivery.CreatedAt < afterCreatedAt || (delivery.CreatedAt == afterCreatedAt && delivery.Transaction.ID <= afterTransactionID) {
			continue
		}
		result = append(result, delivery)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func (stub *deliveryStoreStub) MarkTransactionDelivered(_ context.Context, transactionID string) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.marked = append(stub.marked, transactionID)
	return nil
}

type blockingNotificationSender struct {
	started chan string
	release chan struct{}
}

func (sender *blockingNotificationSender) SendTransaction(ctx context.Context, _ int64, transaction models.Transaction) error {
	sender.started <- transaction.ID
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-sender.release:
		return nil
	}
}

type failingNotificationSender struct {
	mu    sync.Mutex
	calls []string
	fail  map[string]error
}

func (sender *failingNotificationSender) SendTransaction(_ context.Context, _ int64, transaction models.Transaction) error {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	sender.calls = append(sender.calls, transaction.ID)
	return sender.fail[transaction.ID]
}

func delivery(id string, createdAt int64) models.TransactionDelivery {
	return models.TransactionDelivery{Transaction: models.Transaction{ID: id}, TelegramConversationID: 123, CreatedAt: createdAt}
}

func TestTransactionDeliveryWorkerUsesBoundedBatches(t *testing.T) {
	store := &deliveryStoreStub{deliveries: []models.TransactionDelivery{delivery("a", 1), delivery("b", 1), delivery("c", 1), delivery("d", 1)}}
	sender := &blockingNotificationSender{started: make(chan string, 4), release: make(chan struct{}, 4)}
	worker, err := NewTransactionDeliveryWorker(store, sender, time.Minute, 2)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		worker.deliverCycle(context.Background())
		close(done)
	}()

	first := []string{<-sender.started, <-sender.started}
	select {
	case id := <-sender.started:
		t.Fatalf("third concurrent delivery started early: %s", id)
	case <-time.After(20 * time.Millisecond):
	}
	sender.release <- struct{}{}
	sender.release <- struct{}{}
	second := []string{<-sender.started, <-sender.started}
	sender.release <- struct{}{}
	sender.release <- struct{}{}
	<-done

	all := append(first, second...)
	if !sameIDs(all, []string{"a", "b", "c", "d"}) {
		t.Fatalf("sent transactions = %#v", all)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if !sameIDs(store.marked, []string{"a", "b", "c", "d"}) {
		t.Fatalf("marked transactions = %#v", store.marked)
	}
	if !reflect.DeepEqual(store.limits, []int{2, 2, 2}) {
		t.Fatalf("batch limits = %#v", store.limits)
	}
}

func TestTransactionDeliveryWorkerLeavesFailuresSettledWithoutImmediateRetry(t *testing.T) {
	store := &deliveryStoreStub{deliveries: []models.TransactionDelivery{delivery("failed", 1), delivery("sent", 1)}}
	sender := &failingNotificationSender{fail: map[string]error{"failed": errors.New("Telegram unavailable")}}
	worker, err := NewTransactionDeliveryWorker(store, sender, time.Minute, 2)
	if err != nil {
		t.Fatal(err)
	}
	worker.deliverCycle(context.Background())

	sender.mu.Lock()
	calls := append([]string(nil), sender.calls...)
	sender.mu.Unlock()
	if !sameIDs(calls, []string{"failed", "sent"}) || len(calls) != 2 {
		t.Fatalf("send calls = %#v", calls)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if !reflect.DeepEqual(store.marked, []string{"sent"}) {
		t.Fatalf("marked transactions = %#v", store.marked)
	}
}

func TestNewTransactionDeliveryWorkerValidatesConfiguration(t *testing.T) {
	store := &deliveryStoreStub{}
	sender := &failingNotificationSender{}
	if _, err := NewTransactionDeliveryWorker(store, sender, 0, 1); err == nil {
		t.Fatal("zero interval was accepted")
	}
	if _, err := NewTransactionDeliveryWorker(store, sender, time.Minute, 0); err == nil {
		t.Fatal("zero concurrency was accepted")
	}
}

func sameIDs(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	counts := make(map[string]int, len(actual))
	for _, id := range actual {
		counts[id]++
	}
	for _, id := range expected {
		counts[id]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}
