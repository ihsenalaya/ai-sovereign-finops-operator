CREATE TABLE IF NOT EXISTS govar_tenants (
  tenant_id TEXT PRIMARY KEY,
  budget_eur DOUBLE PRECISION NOT NULL DEFAULT 0,
  settled_eur DOUBLE PRECISION NOT NULL DEFAULT 0,
  reserved_eur DOUBLE PRECISION NOT NULL DEFAULT 0,
  active_reservations INTEGER NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS govar_reservations (
  request_id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  selected_deployment TEXT NOT NULL,
  reserved_cost DOUBLE PRECISION NOT NULL,
  actual_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
  policy_version TEXT NOT NULL,
  pricing_version TEXT NOT NULL,
  reservation_mode TEXT NOT NULL,
  risk_level TEXT NOT NULL,
  expiry TIMESTAMPTZ NOT NULL,
  settled BOOLEAN NOT NULL DEFAULT FALSE,
  canceled BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS govar_settlements (
  settlement_id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
