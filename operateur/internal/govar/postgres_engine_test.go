package govar

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPostgresEngineRejectsEmptyURL(t *testing.T) {
	if _, err := NewPostgresEngine(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty database url")
	}
}

func TestPostgresEngineRejectsUnreconciledLegacyFloatLedger(t *testing.T) {
	url := os.Getenv("GOVAR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("GOVAR_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `DROP TABLE IF EXISTS govar_inbox,govar_outbox,govar_settlements,govar_reservations,govar_tenants,govar_schema_migrations CASCADE;
CREATE TABLE govar_tenants (tenant_id TEXT PRIMARY KEY,budget_eur DOUBLE PRECISION NOT NULL DEFAULT 0,settled_eur DOUBLE PRECISION NOT NULL DEFAULT 0,reserved_eur DOUBLE PRECISION NOT NULL DEFAULT 0,active_reservations INTEGER NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
INSERT INTO govar_tenants(tenant_id,budget_eur) VALUES ('legacy',1.25);`)
	if err != nil {
		t.Fatal(err)
	}
	if engine, err := NewPostgresEngine(ctx, url); err == nil {
		engine.Close()
		t.Fatal("service accepted unreconciled legacy floating-point ledger")
	}
	_, _ = conn.Exec(ctx, `DROP TABLE IF EXISTS govar_inbox,govar_outbox,govar_settlements,govar_reservations,govar_tenants,govar_schema_migrations CASCADE`)
}

func TestPostgresEngineLifecycle(t *testing.T) {
	url := os.Getenv("GOVAR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("GOVAR_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	engine, err := NewPostgresEngine(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	if _, err := engine.pool.Exec(ctx, `TRUNCATE govar_inbox,govar_outbox,govar_reservations,govar_tenants`); err != nil {
		t.Fatal(err)
	}
	admit, err := engine.Admit(admitRequest("pg-r1", testTenant, testWorkload), defaultBudget(), defaultRouting(), defaultCandidates())
	if err != nil || admit.Decision != DecisionAdmit {
		t.Fatalf("admit=(%+v,%v)", admit, err)
	}
	duplicate, err := engine.Admit(admitRequest("pg-r1", testTenant, testWorkload), defaultBudget(), defaultRouting(), defaultCandidates())
	if err != nil || duplicate.ReasonCode != ReasonDuplicateRequest {
		t.Fatalf("exact duplicate=(%+v,%v)", duplicate, err)
	}
	conflict := admitRequest("pg-r1", testTenant, testWorkload)
	conflict.MaxOutputTokens++
	if _, err := engine.Admit(conflict, defaultBudget(), defaultRouting(), defaultCandidates()); err == nil {
		t.Fatal("conflicting PostgreSQL duplicate was accepted")
	}
	if _, code, err := engine.Dispatch(dispatchRequest("pg-r1", "pg-d1", admit.ProviderAttemptID, DispatchClaimed, testTenant, testWorkload)); err != nil || code != ReasonDispatchClaimed {
		t.Fatalf("claim=(%s,%v)", code, err)
	}
	if _, code, err := engine.Settle(settleRequest("pg-r1", "pg-s1", 2_000, 1, false, testTenant, testWorkload)); err != nil || code != ReasonProvisionalSettlement {
		t.Fatalf("settle=(%s,%v)", code, err)
	}
	if _, code, err := engine.Settle(settleRequest("pg-r1", "pg-s2", 2_000, 1, true, testTenant, testWorkload)); err != nil || code != ReasonFinalized {
		t.Fatalf("finality=(%s,%v)", code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 2_000, 0, 99_998_000, 0)
	second, err := engine.Admit(admitRequest("pg-missing-outbox", testTenant, testWorkload), defaultBudget(), defaultRouting(), defaultCandidates())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.pool.Exec(ctx, `DELETE FROM govar_outbox WHERE request_id='pg-missing-outbox'`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := engine.Dispatch(dispatchRequest("pg-missing-outbox", "pg-missing-outbox-claim", second.ProviderAttemptID, DispatchClaimed, testTenant, testWorkload)); err == nil {
		t.Fatal("dispatch succeeded although outbox compare-and-swap affected no row")
	}
}
