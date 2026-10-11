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

type connectedUserListerStub struct {
	users []models.User
	err   error
}

func (stub *connectedUserListerStub) ListConnectedUsers(context.Context) ([]models.User, error) {
	return stub.users, stub.err
}

type transactionSynchronizerStub struct {
	mu      sync.Mutex
	userIDs []models.UserID
	fail    map[models.UserID]error
}

func (stub *transactionSynchronizerStub) SyncUser(_ context.Context, userID models.UserID, baseline bool) error {
	if baseline {
		return errors.New("poller requested a baseline sync")
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.userIDs = append(stub.userIDs, userID)
	return stub.fail[userID]
}

func (stub *transactionSynchronizerStub) IDs() []models.UserID {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return append([]models.UserID(nil), stub.userIDs...)
}

func TestTransactionPollerReplacesSnapshotAndContinuesAfterUserFailure(t *testing.T) {
	users := &connectedUserListerStub{users: []models.User{{ID: "first"}, {ID: "second"}}}
	synchronizer := &transactionSynchronizerStub{fail: map[models.UserID]error{"first": errors.New("unavailable")}}
	poller, err := NewTransactionPoller(users, synchronizer, time.Minute, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := poller.refreshSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	poller.pollSnapshot(context.Background())
	if ids := synchronizer.IDs(); !reflect.DeepEqual(ids, []models.UserID{"first", "second"}) {
		t.Fatalf("polled users = %#v", ids)
	}

	users.users = []models.User{{ID: "third"}}
	if err := poller.refreshSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(poller.userIDs, []models.UserID{"third"}) {
		t.Fatalf("snapshot = %#v", poller.userIDs)
	}
}

func TestTransactionPollerKeepsSnapshotWhenRefreshFails(t *testing.T) {
	users := &connectedUserListerStub{users: []models.User{{ID: "existing"}}}
	poller, err := NewTransactionPoller(users, &transactionSynchronizerStub{}, time.Minute, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := poller.refreshSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	users.err = errors.New("database unavailable")
	if err := poller.refreshSnapshot(context.Background()); err == nil {
		t.Fatal("expected refresh failure")
	}
	if !reflect.DeepEqual(poller.userIDs, []models.UserID{"existing"}) {
		t.Fatalf("snapshot changed after failure: %#v", poller.userIDs)
	}
}

func TestTransactionPollerRunsImmediately(t *testing.T) {
	users := &connectedUserListerStub{users: []models.User{{ID: "user"}}}
	synchronizer := &transactionSynchronizerStub{}
	poller, err := NewTransactionPoller(users, synchronizer, time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		poller.Run(ctx)
		close(done)
	}()

	deadline := time.After(time.Second)
	for len(synchronizer.IDs()) == 0 {
		select {
		case <-deadline:
			t.Fatal("poller did not run immediately")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	<-done
}
