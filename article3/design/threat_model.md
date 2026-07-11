# Threat Model

## Scope

The threat model for Article 3 focuses on governance and budget-control correctness, not on proving general security of all infrastructure components.

## Threats Considered

### T1. Budget Exhaustion by Burst Concurrency

Many concurrent requests from one tenant may individually appear admissible if only settled spend is considered.

Mitigation target:

- in-flight reservation accounting

### T2. Delayed Settlement Blindness

Telemetry lag can cause the control plane to underestimate active liability.

Mitigation target:

- reserve-settle ledger and expiry rules

### T3. Governance Bypass by Cheaper Non-Compliant Route

A low-cost model may violate sovereignty or provider restrictions.

Mitigation target:

- hard filter before optimization

### T4. Drift-Induced Under-Reservation

Token length distribution can shift over time, invalidating calibrated reservation levels.

Mitigation target:

- drift alarms and conservative fallback

Current scaffold note:

- the research predictor already includes a simple mean-ratio drift detector
- the admission layer can abstain when `BlockOnDrift` is enabled

### T5. Duplicate Settlement Events

Retries or repeated telemetry delivery may double-charge a tenant.

Mitigation target:

- idempotent settlement keys

### T6. Partial Failure Between Reserve and Dispatch

The system may reserve budget but fail before dispatch, or dispatch without durable settlement metadata.

Mitigation target:

- atomic reservation protocol and expiry cleanup

## Out of Scope

- cryptographic compromise of cloud providers
- legal sufficiency of compliance claims
- adversarial prompt-content attacks beyond budget and routing implications
- full Byzantine telemetry forgery model
