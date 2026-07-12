#!/usr/bin/env python3
"""Reproduce the pinned R55 Solo.io quota-management source audit."""

from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
import tempfile
from pathlib import Path


REPOSITORY = "https://github.com/day0ops/quota-management.git"
COMMIT = "c72d26a7f74f761a9f871b91b8c320db0825db25"
FILES = {
    "internal/budget/service.go": "6b488795419f489382018fdc17bc9ccdf16962614f427c603c13a2c3fdca9af6",
    "internal/db/repository.go": "cc502b40873ed12ddffefb994a7117a95a3e177faa9687d02857403c2449ea9f",
    "internal/models/models.go": "17f9185f931a1a3ee3964e81d617aeb0048c71296d3d95620c041f4d07f09af1",
    "docs/ARCH.md": "210f69bc71fe26abff43a4dca63aac1ba56dcb7b8499ad42d28ed256d5420564",
    "docs/DESIGN.md": "8027f92e9431a35899852c8f227b412a9fb4efc2d1286a694d23f0493586fb4b",
}


def run(*args: str, cwd: Path) -> str:
    proc = subprocess.run(args, cwd=cwd, check=True, text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return proc.stdout.strip()


def audit(source: Path) -> dict[str, object]:
    actual_commit = run("git", "rev-parse", "HEAD", cwd=source)
    assert actual_commit == COMMIT, (actual_commit, COMMIT)
    texts: dict[str, str] = {}
    hashes: dict[str, str] = {}
    for name, expected in FILES.items():
        payload = (source / name).read_bytes()
        actual = hashlib.sha256(payload).hexdigest()
        assert actual == expected, (name, actual, expected)
        hashes[name] = actual
        texts[name] = payload.decode("utf-8")

    service = texts["internal/budget/service.go"]
    repository = texts["internal/db/repository.go"]
    models = texts["internal/models/models.go"]
    architecture = texts["docs/ARCH.md"]
    checks = {
        "atomic_check_and_reserve": "CheckAndReserveBudget" in service and "GetEnabledBudgetsForUpdate" in service,
        "pending_spend": "pending_usage_usd" in repository and "IncrementPendingUsageInTx" in repository,
        "expiry_deletes_hold": "DELETE FROM request_reservations WHERE expires_at <= NOW()" in repository
                               and "pending_usage_usd = GREATEST(0, pending_usage_usd - $2)" in repository,
        "late_usage_without_reservation_not_charged": "reservation not found, cannot decrement budgets" in service,
        "settlement_load_precedes_transaction": service.index("GetReservationByRequestID(ctx, requestID)")
                                                < service.index("BeginTx(ctx)", service.index("func (s *Service) DecrementBudgets")),
        "usage_insert_has_no_request_conflict_clause": "INSERT INTO usage_records" in repository
                                                       and "ON CONFLICT" not in repository[repository.index("func (r *Repository) CreateUsageRecordInTx"):repository.index("func (r *Repository) GetEnabledBudgets")],
        "usage_history_pruned_to_30": "LIMIT 30" in repository,
        "floating_point_money": "BudgetAmountUSD     float64" in models and "PendingUsageUSD     float64" in models,
        "demo_fail_open_documented": "requests are allowed through (fail-open)" in architecture,
    }
    assert all(checks.values()), checks
    return {
        "schema_version": 1,
        "repository": REPOSITORY,
        "commit": COMMIT,
        "file_sha256": hashes,
        "checks": checks,
        "inference": "Because DecrementBudgets reads the reservation before opening its charge transaction, usage insertion has no request-level conflict guard, and deleting an already-deleted reservation is not checked for one affected row, two concurrent distinct settlement deliveries can both apply a charge. This is a source-level concurrency inference to be tested by the faithful baseline harness, not a reported upstream result.",
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, help="existing checkout at the pinned commit")
    args = parser.parse_args()
    if args.source:
        result = audit(args.source.resolve())
    else:
        with tempfile.TemporaryDirectory(prefix="article3-r55-") as temp:
            source = Path(temp)
            run("git", "init", "-q", cwd=source)
            run("git", "remote", "add", "origin", REPOSITORY, cwd=source)
            run("git", "fetch", "-q", "--depth", "1", "origin", COMMIT, cwd=source)
            run("git", "checkout", "-q", "--detach", "FETCH_HEAD", cwd=source)
            result = audit(source)
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
