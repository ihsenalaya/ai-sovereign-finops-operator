# GOV-AR Algorithm

## Informal Overview

GOV-AR is a two-layer method:

1. hard governance filter
   - remove infeasible providers and models

2. risk-bounded admission and routing
   - estimate a reservation for each feasible model
   - compare expected utility against tenant budget and queue pressure
   - choose `admit`, `queue`, `reject`, or `abstain`

## Inputs

For request `r` from tenant `t`:

- request features `x_r`
- compliant model set `M_t(r)`
- tenant budget state
- settled and reserved ledger state
- latency and reliability observations
- quality priors and optional application-specific scores
- output token prediction distribution for each candidate model

## Core Quantities

For each candidate model `m`, GOV-AR computes:

- `u(m, r)`:
  expected utility or score
- `c_in(m, r)`:
  known prompt-side input cost
- `C_out(m, r)`:
  random output cost
- `R_q(m, r)`:
  reservation amount at target quantile or calibrated bound
- `liability_t`:
  current in-flight reserved amount for tenant `t`

The provisional total budget impact of admitting `r` on `m` is:

`reservation_total(m, r) = c_in(m, r) + R_q(m, r)`

## Candidate Evaluation

For each feasible model `m`:

1. predict output token distribution
2. compute reservation bound
3. check whether remaining budget supports the reservation
4. compute a utility-adjusted score
5. penalize models with weak calibration, stale telemetry, or drift alarms
6. abstain instead of forcing a weak-evidence governed decision when required evidence is missing

## Decision Policy

### Strict Mode

Admit model `m*` only if:

- governance constraints hold
- reservation fits entirely in available tenant budget
- guardrails are satisfied

Else:

- queue if short-term release is plausible
- otherwise reject or abstain

### Risk-Bounded Mode

Admit model `m*` only if:

- governance constraints hold
- risk-adjusted reservation fits tenant risk budget
- portfolio exposure remains within configured bounds
- score exceeds baseline safe action

Else:

- queue if expected near-term feasibility improves
- abstain if calibration is weak
- reject if policy or budget makes service unsafe

## Atomic Reserve-Dispatch-Settle Pattern

The algorithm depends on a three-stage pattern:

1. reserve
   - atomically write request reservation in the tenant ledger

2. dispatch
   - send request only after successful reservation commit

3. settle
   - reconcile actual usage when delayed telemetry arrives

This is the main conceptual upgrade over the current operator.

## Conservative Fallbacks

If any of the following is insufficient:

- telemetry freshness
- calibration quality
- route confidence
- provider evidence

GOV-AR degrades conservatively by:

- using a cheaper model
- reserving a larger upper bound
- queueing
- abstaining
- rejecting

## Current Research Scaffold Status

The initial `article3/` code scaffold already includes:

- empirical quantile reservation bounds
- mean-plus-standard-deviation reservation bounds
- tenant settled versus reserved budget state
- per-request reservation records
- idempotent settlement by event identifier
- expiry-based reservation release
- a first decision layer distinguishing `admit`, `queue`, `reject`, and `abstain`
- a minimal replay engine with delayed settlement to test admission-plus-ledger interactions
- replay-time integration of reservation policy selection

## Pseudocode Sketch

```text
for request r from tenant t:
  feasible <- governance_filter(r, tenant_policy[t], model_catalog)
  if feasible is empty:
    return abstain(no_compliant_model)

  candidates <- []
  for m in feasible:
    pred <- predict_output_distribution(r, m)
    reserve <- reservation_bound(pred, mode, alpha_t, calibration_state[m])
    if not budget_feasible(t, reserve):
      continue
    score <- utility_score(r, m, reserve, telemetry, quality, reliability)
    score <- apply_drift_and_confidence_penalties(score)
    candidates.append((m, reserve, score))

  if candidates is empty:
    return queue_or_reject_or_abstain(t, r)

  best <- argmax score over candidates
  if not atomic_reserve(t, r, best.reserve, best.model):
    return queue_or_reject(t, r)

  dispatch(r, best.model)
  return admit(best.model, best.reserve)
```

## Expected Experimental Knobs

- strict versus risk-bounded mode
- reservation quantile
- reservation z-score
- queue timeout
- drift penalty strength
- model utility weights
- fallback policy under missing evidence
