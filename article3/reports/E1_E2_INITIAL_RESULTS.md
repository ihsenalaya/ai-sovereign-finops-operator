# E1/E2 Initial Results

## Scope

These are early deterministic scaffold runs produced by the Article 3 replay engine.

They are not yet publication-grade experiments. Their purpose is to validate:

- replay execution
- comparative reservation policies
- machine-readable output generation

## E1 Budget Delay

Compared variants:

- `quantile`
- `mean_std`

Observed summaries:

- `quantile`
  - admitted: 2
  - queued: 0
  - settled total: 12
  - reserved total: 14
  - slack total: 2
  - overshoot total: 0

- `mean_std`
  - admitted: 1
  - queued: 1
  - settled total: 7
  - reserved total: 8
  - slack total: 1
  - overshoot total: 0

Initial interpretation:

- `quantile` admitted both requests under delayed settlement
- `mean_std` was conservative enough to defer the second request
- E1 now exposes a real admission trade-off between throughput and reservation conservatism

## E2 Multitenant

Compared variants:

- `quantile`
- `mean_std`

Observed summaries:

- `quantile`
  - admitted: 3
  - queued: 1
  - settled total: 15
  - reserved total: 15
  - slack total: 0.5
  - overshoot total: 0.5

- `mean_std`
  - admitted: 3
  - queued: 1
  - settled total: 15
  - reserved total: 16.5
  - slack total: 1.5
  - overshoot total: 0

Initial interpretation:

- both policies admitted the same number of requests in this trace
- `quantile` is tighter but incurred non-zero overshoot
- `mean_std` avoided overshoot at the cost of higher slack
- E2 now exposes the intended safety-versus-efficiency tension

## Next Steps

- expand traces so quantile and mean-std produce different admission outcomes
- compute overshoot and slack metrics explicitly
- add larger multi-tenant synthetic campaigns for E1/E2
