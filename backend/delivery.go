package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yohanc3/steer/models"
)

type transactionDeliveryStore interface {
	ListSettledTransactionDeliveries(context.Context, int64, string, int) ([]models.TransactionDelivery, error)
	MarkTransactionDelivered(context.Context, string) error
}

type transactionNotificationSender interface {
	SendTransaction(context.Context, int64, models.Transaction) error
}

// TransactionDeliveryWorker drains settled transactions in bounded concurrent batches.
type TransactionDeliveryWorker struct {
	store       transactionDeliveryStore
	sender      transactionNotificationSender
	interval    time.Duration
	concurrency int
}

func NewTransactionDeliveryWorker(store transactionDeliveryStore, sender transactionNotificationSender, interval time.Duration, concurrency int) (*TransactionDeliveryWorker, error) {
	if store == nil {
		return nil, fmt.Errorf("transaction delivery store is required")
	}
	if sender == nil {
		return nil, fmt.Errorf("transaction notification sender is required")
	}
	if interval <= 0 {
		return nil, fmt.Errorf("delivery interval must be greater than zero")
	}
	if concurrency <= 0 {
		return nil, fmt.Errorf("delivery concurrency must be greater than zero")
	}
	return &TransactionDeliveryWorker{store: store, sender: sender, interval: interval, concurrency: concurrency}, nil
}

// Run drains pending work immediately and then once per configured interval.
func (worker *TransactionDeliveryWorker) Run(ctx context.Context) {
	worker.deliverCycle(ctx)
	ticker := time.NewTicker(worker.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			worker.deliverCycle(ctx)
		}
	}
}

func (worker *TransactionDeliveryWorker) deliverCycle(ctx context.Context) {
	var afterCreatedAt int64
	var afterTransactionID string
	for ctx.Err() == nil {
		deliveries, err := worker.store.ListSettledTransactionDeliveries(ctx, afterCreatedAt, afterTransactionID, worker.concurrency)
		if err != nil {
			slog.Log(ctx, slog.LevelError, "list settled transaction deliveries", "error", err)
			return
		}
		if len(deliveries) == 0 {
			return
		}

		worker.deliverBatch(ctx, deliveries)
		last := deliveries[len(deliveries)-1]
		afterCreatedAt = last.CreatedAt
		afterTransactionID = last.Transaction.ID
		if len(deliveries) < worker.concurrency {
			return
		}
	}
}

func (worker *TransactionDeliveryWorker) deliverBatch(ctx context.Context, deliveries []models.TransactionDelivery) {
	var wait sync.WaitGroup
	for _, delivery := range deliveries {
		delivery := delivery
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := worker.sender.SendTransaction(ctx, delivery.TelegramConversationID, delivery.Transaction); err != nil {
				slog.Log(ctx, slog.LevelError, "send transaction notification", "transaction_id", delivery.Transaction.ID, "error", err)
				return
			}
			if err := worker.store.MarkTransactionDelivered(ctx, delivery.Transaction.ID); err != nil {
				slog.Log(ctx, slog.LevelError, "mark transaction notification delivered", "transaction_id", delivery.Transaction.ID, "error", err)
			}
		}()
	}
	wait.Wait()
}
