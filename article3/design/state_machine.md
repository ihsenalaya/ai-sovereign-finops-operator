# State Machine

## Per-Request Lifecycle

Each governed request moves through the following conceptual states.

### 1. `received`

Input metadata has arrived, but no decision has been committed yet.

### 2. `filtered`

Hard governance filtering removes inadmissible models.

Transitions:

- to `abstained` if no compliant route exists
- to `scored` if at least one compliant route exists

### 3. `scored`

Candidate models are scored using:

- quality priors
- observed latency and reliability
- price priors
- tenant budget state
- uncertainty-aware reservation estimate

Transitions:

- to `queued`
- to `rejected`
- to `reserved`
- to `abstained`

### 4. `reserved`

The selected action is `admit`, and a provisional reservation has been atomically written.

Transition:

- to `dispatched`

### 5. `dispatched`

The request has been sent to the selected model endpoint and is now in flight.

Transitions:

- to `settling`
- to `expired`
- to `failed`

### 6. `settling`

Delayed telemetry or gateway evidence provides actual cost and outcome.

Transition:

- to `settled`

### 7. `settled`

Reservation is reconciled against actual cost:

- release unused amount if actual < reserved
- record overshoot if actual > reserved
- mark idempotent completion token

Current scaffold note:

- `SettlementEventID` is the idempotence key already modeled in the Article 3 research ledger

### 8. `queued`

The request was not admitted immediately but may be reconsidered later.

Transitions:

- to `received` on retry
- to `expired`
- to `rejected`

Current scaffold note:

- the replay harness already supports configurable queued retries with delayed re-entry

### 9. `rejected`

The request is denied by policy or capacity rule.

Terminal state.

### 10. `abstained`

The system chooses not to make a confident governed routing decision.

Terminal state.

### 11. `expired`

The queued or in-flight request exceeded a deadline or settlement TTL.

Terminal state after release.

### 12. `failed`

The upstream call or telemetry pipeline failed before clean settlement.

Terminal after compensating release or bounded fallback settlement.

## Per-Tenant Budget Lifecycle

Budget state for tenant `t` is summarized by:

- `settled_spend`
- `reserved_spend`
- `available_spend`
- `risk_budget`
- `drift_status`

Core transitions:

- `reserve`
- `release`
- `settle`
- `expire`
- `recalibrate`

## Design Intent

This state machine is deliberately more explicit than the current operator’s report-and-reroute loop because GOV-AR needs a first-class reserve-settle lifecycle.
