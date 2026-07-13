package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/imperium/ai-sovereign-finops-operator/internal/govar"
)

type durableReconciliationBackend interface {
	ReconcileExpired(context.Context, time.Time) ([]govar.ReconciliationRecord, error)
	PendingReconciliation(context.Context, int) ([]govar.ReconciliationRecord, error)
}

type reconciliationWorkerConfig struct {
	Interval   time.Duration
	QueryLimit int
	Now        func() time.Time
}

func reconciliationWorkerConfigFromEnvironment() reconciliationWorkerConfig {
	config := reconciliationWorkerConfig{Interval: 5 * time.Second, QueryLimit: 100, Now: time.Now}
	if raw := strings.TrimSpace(os.Getenv("GOV_AR_RECONCILIATION_INTERVAL")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed >= time.Second && parsed <= time.Hour {
			config.Interval = parsed
		} else {
			log.Printf("ignoring invalid GOV_AR_RECONCILIATION_INTERVAL %q", raw)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("GOV_AR_RECONCILIATION_QUERY_LIMIT")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= 10_000 {
			config.QueryLimit = parsed
		} else {
			log.Printf("ignoring invalid GOV_AR_RECONCILIATION_QUERY_LIMIT %q", raw)
		}
	}
	return config
}

func startReconciliationWorker(ctx context.Context, backend durableReconciliationBackend, config reconciliationWorkerConfig) {
	if config.Interval <= 0 {
		config.Interval = 5 * time.Second
	}
	if config.QueryLimit <= 0 {
		config.QueryLimit = 100
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	go func() {
		runReconciliationPass(ctx, backend, config)
		ticker := time.NewTicker(config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runReconciliationPass(ctx, backend, config)
			}
		}
	}()
}

func runReconciliationPass(ctx context.Context, backend durableReconciliationBackend, config reconciliationWorkerConfig) {
	if ctx.Err() != nil {
		return
	}
	reconciled, err := backend.ReconcileExpired(ctx, config.Now().UTC())
	if err != nil {
		govarReconciliationRuns.WithLabelValues("error").Inc()
		log.Printf("durable reconciliation pass failed: %v", err)
		return
	}
	pending, err := backend.PendingReconciliation(ctx, config.QueryLimit)
	if err != nil {
		govarReconciliationRuns.WithLabelValues("error").Inc()
		log.Printf("durable reconciliation pending scan failed: %v", err)
		return
	}
	govarReconciledRecords.Add(float64(len(reconciled)))
	govarPendingReconciliation.Set(float64(len(pending)))
	govarReconciliationLastSuccess.Set(float64(config.Now().UTC().Unix()))
	govarReconciliationRuns.WithLabelValues("success").Inc()
}
