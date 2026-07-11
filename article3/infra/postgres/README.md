# PostgreSQL for GOV-AR Admission

This directory contains the Article 3 PostgreSQL bootstrap material for the
operator-side `gov-ar-admission` service.

## Scope

- optional persistent backend for request reservations and settlements
- local developer bootstrap via Docker Compose
- schema initialization for the GOV-AR ledger tables

## Usage

```bash
docker compose -f article3/infra/postgres/docker-compose.yaml up -d
export DATABASE_URL=postgres://govar:govar@127.0.0.1:5432/govar?sslmode=disable
```

Then start the service with:

```bash
cd operateur
go run ./cmd/gov-ar-admission/main.go
```

When `DATABASE_URL` is unset, the service falls back to the in-memory ledger.
