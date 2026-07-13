package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/imperium/ai-sovereign-finops-operator/internal/govar"
)

type fakeReconciliationBackend struct {
	mu             sync.Mutex
	reconcileCalls int
	pendingCalls   int
	reconcileErr   error
	pendingErr     error
}

func (f *fakeReconciliationBackend) ReconcileExpired(context.Context, time.Time) ([]govar.ReconciliationRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reconcileCalls++
	return []govar.ReconciliationRecord{{RequestID: "synthetic-request"}}, f.reconcileErr
}

func (f *fakeReconciliationBackend) PendingReconciliation(context.Context, int) ([]govar.ReconciliationRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pendingCalls++
	return nil, f.pendingErr
}

func (f *fakeReconciliationBackend) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reconcileCalls, f.pendingCalls
}

func TestReconciliationWorkerRunsImmediatelyAndStops(t *testing.T) {
	backend := &fakeReconciliationBackend{}
	ctx, cancel := context.WithCancel(context.Background())
	startReconciliationWorker(ctx, backend, reconciliationWorkerConfig{Interval: 5 * time.Millisecond, QueryLimit: 7, Now: time.Now})
	deadline := time.Now().Add(time.Second)
	for {
		reconcileCalls, pendingCalls := backend.counts()
		if reconcileCalls >= 2 && pendingCalls >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("worker did not run twice: reconcile=%d pending=%d", reconcileCalls, pendingCalls)
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	reconcileCalls, pendingCalls := backend.counts()
	time.Sleep(20 * time.Millisecond)
	afterReconcile, afterPending := backend.counts()
	if afterReconcile != reconcileCalls || afterPending != pendingCalls {
		t.Fatalf("worker continued after cancellation: before=(%d,%d) after=(%d,%d)", reconcileCalls, pendingCalls, afterReconcile, afterPending)
	}
}

func TestReconciliationPassDoesNotScanPendingAfterFailure(t *testing.T) {
	backend := &fakeReconciliationBackend{reconcileErr: errors.New("synthetic failure")}
	runReconciliationPass(context.Background(), backend, reconciliationWorkerConfig{QueryLimit: 5, Now: time.Now})
	reconcileCalls, pendingCalls := backend.counts()
	if reconcileCalls != 1 || pendingCalls != 0 {
		t.Fatalf("unexpected calls after failed reconcile: reconcile=%d pending=%d", reconcileCalls, pendingCalls)
	}
}

func TestReconciliationWorkerConfigurationBounds(t *testing.T) {
	t.Setenv("GOV_AR_RECONCILIATION_INTERVAL", "250ms")
	t.Setenv("GOV_AR_RECONCILIATION_QUERY_LIMIT", "0")
	config := reconciliationWorkerConfigFromEnvironment()
	if config.Interval != 5*time.Second || config.QueryLimit != 100 {
		t.Fatalf("invalid environment values were accepted: %+v", config)
	}
	t.Setenv("GOV_AR_RECONCILIATION_INTERVAL", "10s")
	t.Setenv("GOV_AR_RECONCILIATION_QUERY_LIMIT", "250")
	config = reconciliationWorkerConfigFromEnvironment()
	if config.Interval != 10*time.Second || config.QueryLimit != 250 {
		t.Fatalf("valid environment values were not accepted: %+v", config)
	}
}
