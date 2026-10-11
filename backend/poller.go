package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"yohanc3/steer/models"
)

type connectedUserLister interface {
	ListConnectedUsers(context.Context) ([]models.User, error)
}

type transactionSynchronizer interface {
	SyncUser(context.Context, models.UserID, bool) error
}

// TransactionPoller owns one immutable snapshot of connected user IDs at a time.
// It replaces the whole snapshot on refresh and synchronizes users sequentially.
type TransactionPoller struct {
	users           connectedUserLister
	synchronizer    transactionSynchronizer
	pollInterval    time.Duration
	refreshInterval time.Duration
	userIDs         []models.UserID
}

func NewTransactionPoller(users connectedUserLister, synchronizer transactionSynchronizer, pollInterval, refreshInterval time.Duration) (*TransactionPoller, error) {
	if users == nil {
		return nil, fmt.Errorf("connected user store is required")
	}
	if synchronizer == nil {
		return nil, fmt.Errorf("transaction synchronizer is required")
	}
	if pollInterval <= 0 {
		return nil, fmt.Errorf("poll interval must be greater than zero")
	}
	if refreshInterval <= 0 {
		return nil, fmt.Errorf("queue refresh interval must be greater than zero")
	}
	return &TransactionPoller{users: users, synchronizer: synchronizer, pollInterval: pollInterval, refreshInterval: refreshInterval}, nil
}

// Run loads and polls the first snapshot immediately, then follows both timers.
func (poller *TransactionPoller) Run(ctx context.Context) {
	if err := poller.refreshSnapshot(ctx); err != nil {
		slog.Log(ctx, slog.LevelError, "refresh teller polling queue", "error", err)
	}
	poller.pollSnapshot(ctx)

	pollTicker := time.NewTicker(poller.pollInterval)
	defer pollTicker.Stop()
	refreshTicker := time.NewTicker(poller.refreshInterval)
	defer refreshTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-refreshTicker.C:
			if err := poller.refreshSnapshot(ctx); err != nil {
				slog.Log(ctx, slog.LevelError, "refresh teller polling queue", "error", err)
			}
		case <-pollTicker.C:
			poller.pollSnapshot(ctx)
		}
	}
}

func (poller *TransactionPoller) refreshSnapshot(ctx context.Context) error {
	users, err := poller.users.ListConnectedUsers(ctx)
	if err != nil {
		return err
	}

	next := make([]models.UserID, len(users))
	for i, user := range users {
		next[i] = user.ID
	}
	poller.userIDs = next
	return nil
}

func (poller *TransactionPoller) pollSnapshot(ctx context.Context) {
	for _, userID := range poller.userIDs {
		if err := poller.synchronizer.SyncUser(ctx, userID, false); err != nil {
			slog.Log(ctx, slog.LevelError, "poll teller transactions", "user_id", userID, "error", err)
		}
	}
}
